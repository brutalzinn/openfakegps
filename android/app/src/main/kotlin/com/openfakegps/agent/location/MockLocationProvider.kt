package com.openfakegps.agent.location

import android.content.Context
import android.location.Criteria
import android.location.Location
import android.location.LocationManager
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.util.Log
import com.google.android.gms.location.FusedLocationProviderClient
import com.google.android.gms.location.LocationServices
import com.openfakegps.agent.grpc.LocationUpdateData

class MockLocationProvider(context: Context) {

    companion object {
        private const val TAG = "MockLocationProvider"
        private const val FLOOD_INTERVAL_MS = 100L
        private val PROVIDERS = listOf(
            LocationManager.GPS_PROVIDER,
            LocationManager.NETWORK_PROVIDER
        )
    }

    private val locationManager =
        context.getSystemService(Context.LOCATION_SERVICE) as LocationManager

    private val fusedClient: FusedLocationProviderClient =
        LocationServices.getFusedLocationProviderClient(context)

    @Volatile
    private var isInitialized = false

    private var fusedMockEnabled = false

    private val initializedProviders = mutableSetOf<String>()

    @Volatile
    private var lastUpdate: LocationUpdateData? = null

    private val handler = Handler(Looper.getMainLooper())
    private val floodRunnable = object : Runnable {
        override fun run() {
            lastUpdate?.let { resendLocation(it) }
            if (isInitialized) {
                handler.postDelayed(this, FLOOD_INTERVAL_MS)
            }
        }
    }

    fun initialize(): Boolean {
        var anySuccess = false
        for (provider in PROVIDERS) {
            if (initProvider(provider)) {
                initializedProviders.add(provider)
                anySuccess = true
            }
        }

        // Enable mock mode on FusedLocationProviderClient.
        try {
            fusedClient.setMockMode(true)
                .addOnSuccessListener {
                    fusedMockEnabled = true
                    Log.i(TAG, "FusedLocationProviderClient mock mode enabled")
                }
                .addOnFailureListener { e ->
                    Log.e(TAG, "FusedLocationProviderClient mock mode failed", e)
                }
            anySuccess = true
        } catch (e: SecurityException) {
            Log.e(TAG, "FusedLocationProviderClient: not allowed", e)
        }

        if (anySuccess) {
            isInitialized = true
            handler.removeCallbacks(floodRunnable)
            handler.postDelayed(floodRunnable, FLOOD_INTERVAL_MS)
            Log.i(TAG, "Mock location providers initialized: $initializedProviders")
        }
        return anySuccess
    }

    private fun initProvider(provider: String): Boolean {
        return try {
            locationManager.addTestProvider(
                provider,
                false,  // requiresNetwork
                false,  // requiresSatellite
                false,  // requiresCell
                false,  // hasMonetaryCost
                true,   // supportsAltitude
                true,   // supportsSpeed
                true,   // supportsBearing
                Criteria.POWER_LOW,
                Criteria.ACCURACY_FINE
            )
            locationManager.setTestProviderEnabled(provider, true)
            Log.i(TAG, "Mock provider '$provider' added")
            true
        } catch (e: SecurityException) {
            Log.e(TAG, "Mock provider '$provider': not allowed", e)
            false
        } catch (e: IllegalArgumentException) {
            try {
                locationManager.setTestProviderEnabled(provider, true)
                Log.i(TAG, "Mock provider '$provider' re-enabled")
                true
            } catch (ex: Exception) {
                Log.e(TAG, "Mock provider '$provider': failed to enable", ex)
                false
            }
        }
    }

    fun setLocation(update: LocationUpdateData) {
        if (!isInitialized) {
            Log.w(TAG, "Mock provider not initialized, skipping location update")
            return
        }

        // Only update the data. The flood loop handles all injection
        // to avoid race conditions between gRPC callbacks and the loop.
        lastUpdate = update
    }

    private fun resendLocation(update: LocationUpdateData) {
        val now = System.currentTimeMillis()
        val elapsedNanos = SystemClock.elapsedRealtimeNanos()

        // Inject into LocationManager test providers.
        for (provider in initializedProviders) {
            try {
                val location = buildLocation(provider, update, now, elapsedNanos)
                locationManager.setTestProviderLocation(provider, location)
            } catch (e: SecurityException) {
                Log.e(TAG, "Failed to set mock location on '$provider': disabled", e)
            } catch (e: IllegalArgumentException) {
                Log.e(TAG, "Failed to set mock location on '$provider': invalid state", e)
            }
        }

        // Inject into FusedLocationProviderClient.
        if (fusedMockEnabled) {
            try {
                val fusedLocation = buildLocation("fused", update, now, elapsedNanos)
                fusedClient.setMockLocation(fusedLocation)
            } catch (e: Exception) {
                Log.e(TAG, "Failed to set fused mock location", e)
            }
        }
    }

    private fun buildLocation(
        provider: String,
        update: LocationUpdateData,
        now: Long,
        elapsedNanos: Long
    ): Location {
        return Location(provider).apply {
            latitude = update.latitude
            longitude = update.longitude
            speed = update.speed
            bearing = update.bearing
            accuracy = update.accuracy
            altitude = update.altitude
            time = now
            elapsedRealtimeNanos = elapsedNanos

            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                bearingAccuracyDegrees = 1.0f
                speedAccuracyMetersPerSecond = 0.5f
                verticalAccuracyMeters = update.accuracy
            }
        }
    }

    fun cleanup() {
        if (!isInitialized) return
        handler.removeCallbacks(floodRunnable)
        lastUpdate = null

        // Disable fused mock mode.
        if (fusedMockEnabled) {
            try {
                fusedClient.setMockMode(false)
                fusedMockEnabled = false
            } catch (e: Exception) {
                Log.w(TAG, "Error disabling fused mock mode", e)
            }
        }

        for (provider in initializedProviders) {
            try {
                locationManager.setTestProviderEnabled(provider, false)
                locationManager.removeTestProvider(provider)
                Log.i(TAG, "Mock provider '$provider' removed")
            } catch (e: Exception) {
                Log.w(TAG, "Error cleaning up mock provider '$provider'", e)
            }
        }
        initializedProviders.clear()
        isInitialized = false
    }
}
