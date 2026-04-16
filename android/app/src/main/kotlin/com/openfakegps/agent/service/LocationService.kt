package com.openfakegps.agent.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import android.util.Log
import androidx.core.app.NotificationCompat
import com.openfakegps.agent.MainActivity
import com.openfakegps.agent.R
import com.openfakegps.agent.grpc.AgentCallback
import com.openfakegps.agent.grpc.AgentClient
import com.openfakegps.agent.grpc.LocationUpdateData
import com.openfakegps.agent.location.MockLocationProvider
import com.openfakegps.agent.ui.MainViewModel

class LocationService : Service(), AgentCallback {

    companion object {
        private const val TAG = "LocationService"
        private const val NOTIFICATION_ID = 1001
        private const val CHANNEL_ID = "gps_simulation_channel"

        const val ACTION_START = "com.openfakegps.agent.ACTION_START"
        const val ACTION_STOP = "com.openfakegps.agent.ACTION_STOP"

        const val EXTRA_HOST = "extra_host"
        const val EXTRA_PORT = "extra_port"
        const val EXTRA_AGENT_ID = "extra_agent_id"
    }

    private var agentClient: AgentClient? = null
    private var mockLocationProvider: MockLocationProvider? = null
    private var wakeLock: PowerManager.WakeLock? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> {
                val host = intent.getStringExtra(EXTRA_HOST) ?: "10.0.2.2"
                val port = intent.getIntExtra(EXTRA_PORT, 50051)
                val agentId = intent.getStringExtra(EXTRA_AGENT_ID) ?: "unknown"

                startForeground(NOTIFICATION_ID, buildNotification())
                acquireWakeLock()
                startSimulation(host, port, agentId)
            }
            ACTION_STOP -> {
                stopSimulation()
                stopForeground(STOP_FOREGROUND_REMOVE)
                stopSelf()
            }
        }
        return START_STICKY
    }

    private fun startSimulation(host: String, port: Int, agentId: String) {
        // Initialize mock location provider
        mockLocationProvider = MockLocationProvider(this)
        val providerReady = mockLocationProvider?.initialize() ?: false
        if (!providerReady) {
            Log.e(TAG, "Failed to initialize mock location provider")
            MainViewModel.shared?.updateConnectionStatus("Mock location not enabled", false)
        }

        // Create and connect gRPC client
        agentClient = AgentClient(
            host = host,
            port = port,
            agentId = agentId,
            deviceName = Build.DEVICE,
            deviceModel = Build.MODEL,
            callback = this
        )
        agentClient?.connect()

        MainViewModel.shared?.updateConnectionStatus("Connecting...", false)
    }

    private fun stopSimulation() {
        agentClient?.disconnect()
        agentClient = null

        mockLocationProvider?.cleanup()
        mockLocationProvider = null

        releaseWakeLock()

        MainViewModel.shared?.reset()
    }

    // -- AgentCallback --

    override fun onConnected() {
        Log.i(TAG, "Connected to server")
        MainViewModel.shared?.updateConnectionStatus("Connected", true)
    }

    override fun onDisconnected() {
        Log.i(TAG, "Disconnected from server")
        MainViewModel.shared?.updateConnectionStatus("Disconnected", false)
    }

    override fun onLocationUpdate(update: LocationUpdateData) {
        mockLocationProvider?.setLocation(update)
        MainViewModel.shared?.updateSimulationId(update.simulationId)
        MainViewModel.shared?.updateLocation(
            lat = update.latitude,
            lon = update.longitude,
            speed = update.speed,
            bearing = update.bearing
        )
    }

    override fun onSimulationCommand(action: String, simulationId: String) {
        Log.i(TAG, "Simulation command: $action for $simulationId")
        when (action) {
            "SIMULATION_ACTION_START" -> {
                MainViewModel.shared?.updateSimulationId(simulationId)
                MainViewModel.shared?.updateConnectionStatus("Simulating", true)
            }
            "SIMULATION_ACTION_STOP" -> {
                MainViewModel.shared?.updateSimulationId(null)
                MainViewModel.shared?.updateConnectionStatus("Connected", true)
            }
            "SIMULATION_ACTION_PAUSE" -> {
                MainViewModel.shared?.updateConnectionStatus("Paused", true)
            }
            "SIMULATION_ACTION_RESUME" -> {
                MainViewModel.shared?.updateConnectionStatus("Simulating", true)
            }
        }
    }

    override fun onError(error: String) {
        Log.e(TAG, "Agent error: $error")
        MainViewModel.shared?.updateConnectionStatus("Error: $error", false)
    }

    // -- Notification --

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            getString(R.string.notification_channel),
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "Shows when GPS simulation is active"
        }
        val manager = getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(channel)
    }

    private fun buildNotification(): Notification {
        val pendingIntent = PendingIntent.getActivity(
            this, 0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(getString(R.string.notification_title))
            .setContentText(getString(R.string.notification_text))
            .setSmallIcon(android.R.drawable.ic_menu_mylocation)
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build()
    }

    // -- WakeLock --

    private fun acquireWakeLock() {
        val pm = getSystemService(Context.POWER_SERVICE) as PowerManager
        wakeLock = pm.newWakeLock(
            PowerManager.PARTIAL_WAKE_LOCK,
            "OpenFakeGPS::LocationServiceLock"
        ).apply {
            acquire(4 * 60 * 60 * 1000L) // 4 hours max
        }
    }

    private fun releaseWakeLock() {
        try {
            wakeLock?.let {
                if (it.isHeld) it.release()
            }
        } catch (e: Exception) {
            Log.w(TAG, "Error releasing wake lock", e)
        }
        wakeLock = null
    }

    override fun onDestroy() {
        stopSimulation()
        super.onDestroy()
    }
}
