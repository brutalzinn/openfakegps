package com.openfakegps.agent.location

import android.content.Context
import android.location.Location
import android.location.LocationManager
import android.location.provider.ProviderProperties
import android.os.Build
import android.os.SystemClock
import android.util.Log
import com.openfakegps.agent.grpc.LocationUpdateData

class MockLocationProvider(context: Context) {

    companion object {
        private const val TAG = "MockLocationProvider"
        private const val PROVIDER = LocationManager.GPS_PROVIDER
    }

    private val locationManager =
        context.getSystemService(Context.LOCATION_SERVICE) as LocationManager

    @Volatile
    private var isInitialized = false

    fun initialize(): Boolean {
        return try {
            locationManager.addTestProvider(
                PROVIDER,
                false,  // requiresNetwork
                false,  // requiresSatellite
                false,  // requiresCell
                false,  // hasMonetaryCost
                true,   // supportsAltitude
                true,   // supportsSpeed
                true,   // supportsBearing
                ProviderProperties.POWER_USAGE_LOW,
                ProviderProperties.ACCURACY_FINE
            )
            locationManager.setTestProviderEnabled(PROVIDER, true)
            isInitialized = true
            Log.i(TAG, "Mock location provider initialized")
            true
        } catch (e: SecurityException) {
            Log.e(TAG, "Mock locations not enabled in developer settings", e)
            isInitialized = false
            false
        } catch (e: IllegalArgumentException) {
            // Provider may already exist; try to enable it
            try {
                locationManager.setTestProviderEnabled(PROVIDER, true)
                isInitialized = true
                Log.i(TAG, "Mock location provider re-enabled")
                true
            } catch (ex: Exception) {
                Log.e(TAG, "Failed to enable mock provider", ex)
                isInitialized = false
                false
            }
        }
    }

    fun setLocation(update: LocationUpdateData) {
        if (!isInitialized) {
            Log.w(TAG, "Mock provider not initialized, skipping location update")
            return
        }

        try {
            val location = Location(PROVIDER).apply {
                latitude = update.latitude
                longitude = update.longitude
                speed = update.speed
                bearing = update.bearing
                accuracy = update.accuracy
                altitude = update.altitude
                time = if (update.timestampMs > 0) update.timestampMs else System.currentTimeMillis()
                elapsedRealtimeNanos = SystemClock.elapsedRealtimeNanos()

                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                    bearingAccuracyDegrees = 1.0f
                    speedAccuracyMetersPerSecond = 0.5f
                    verticalAccuracyMeters = update.accuracy
                }
            }

            locationManager.setTestProviderLocation(PROVIDER, location)
        } catch (e: SecurityException) {
            Log.e(TAG, "Failed to set mock location: mock locations disabled", e)
        } catch (e: IllegalArgumentException) {
            Log.e(TAG, "Failed to set mock location: invalid provider state", e)
        }
    }

    fun cleanup() {
        if (!isInitialized) return
        try {
            locationManager.setTestProviderEnabled(PROVIDER, false)
            locationManager.removeTestProvider(PROVIDER)
            Log.i(TAG, "Mock location provider removed")
        } catch (e: Exception) {
            Log.w(TAG, "Error cleaning up mock provider", e)
        } finally {
            isInitialized = false
        }
    }
}
