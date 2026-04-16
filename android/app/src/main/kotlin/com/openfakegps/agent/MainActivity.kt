package com.openfakegps.agent

import android.Manifest
import android.app.AppOpsManager
import android.content.Intent
import android.content.SharedPreferences
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import android.view.View
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.lifecycle.ViewModelProvider
import com.openfakegps.agent.databinding.ActivityMainBinding
import com.openfakegps.agent.service.LocationService
import com.openfakegps.agent.ui.MainViewModel
import java.util.UUID

class MainActivity : AppCompatActivity() {

    private lateinit var binding: ActivityMainBinding
    private lateinit var viewModel: MainViewModel
    private lateinit var prefs: SharedPreferences
    private var isServiceRunning = false

    private val permissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions()
    ) { _ ->
        updateSetupCard()
        autoConnectIfReady()
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        viewModel = ViewModelProvider(this)[MainViewModel::class.java]
        MainViewModel.shared = viewModel

        prefs = getSharedPreferences("openfakegps", MODE_PRIVATE)

        setupAgentId()
        setupUI()
        setupSetupCard()
        observeViewModel()
        requestPermissions()
        autoConnectIfReady()
    }

    override fun onResume() {
        super.onResume()
        updateSetupCard()
    }

    override fun onPause() {
        super.onPause()
        saveServerConfig()
    }

    private fun setupAgentId() {
        var agentId = prefs.getString("agent_id", null)
        if (agentId == null) {
            agentId = "${Build.MODEL}-${UUID.randomUUID().toString().take(8)}"
            prefs.edit().putString("agent_id", agentId).apply()
        }
        binding.textAgentId.text = agentId
    }

    private fun setupUI() {
        binding.inputHost.setText(prefs.getString("server_host", "10.0.2.2"))
        binding.inputPort.setText(prefs.getString("server_port", "50051"))

        binding.buttonConnect.setOnClickListener {
            if (isServiceRunning) {
                stopLocationService()
            } else {
                startLocationService()
            }
        }
    }

    private fun setupSetupCard() {
        binding.buttonGrantPermissions.setOnClickListener {
            requestPermissions()
        }

        binding.buttonDevOptions.setOnClickListener {
            try {
                startActivity(Intent(Settings.ACTION_APPLICATION_DEVELOPMENT_SETTINGS))
            } catch (e: Exception) {
                Toast.makeText(this, "Could not open Developer Options", Toast.LENGTH_SHORT).show()
            }
        }

        // Hide notification row on older Android versions
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) {
            binding.rowNotification.visibility = View.GONE
        }

        updateSetupCard()
    }

    private fun updateSetupCard() {
        val locationGranted = ContextCompat.checkSelfPermission(
            this, Manifest.permission.ACCESS_FINE_LOCATION
        ) == PackageManager.PERMISSION_GRANTED

        val notificationGranted = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            ContextCompat.checkSelfPermission(
                this, Manifest.permission.POST_NOTIFICATIONS
            ) == PackageManager.PERMISSION_GRANTED
        } else {
            true
        }

        val mockLocationEnabled = isMockLocationEnabled()

        // Location
        updateIndicator(
            binding.indicatorLocation,
            binding.textLocationStatus,
            locationGranted,
            getString(R.string.status_granted),
            getString(R.string.status_denied)
        )

        // Notifications
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            updateIndicator(
                binding.indicatorNotification,
                binding.textNotificationStatus,
                notificationGranted,
                getString(R.string.status_granted),
                getString(R.string.status_denied)
            )
        }

        // Mock location
        updateIndicator(
            binding.indicatorMockLocation,
            binding.textMockLocationStatus,
            mockLocationEnabled,
            getString(R.string.status_selected),
            getString(R.string.status_not_selected)
        )

        val allReady = locationGranted && notificationGranted && mockLocationEnabled

        // Show/hide hint and buttons based on readiness
        if (allReady) {
            binding.textSetupHint.text = getString(R.string.setup_ready)
            binding.buttonGrantPermissions.visibility = View.GONE
            binding.buttonDevOptions.visibility = View.GONE
        } else {
            binding.textSetupHint.text = getString(R.string.mock_location_hint)
            binding.buttonGrantPermissions.visibility =
                if (locationGranted && notificationGranted) View.GONE else View.VISIBLE
            binding.buttonDevOptions.visibility =
                if (mockLocationEnabled) View.GONE else View.VISIBLE
        }
    }

    private fun updateIndicator(indicator: View, statusText: android.widget.TextView, ok: Boolean, okLabel: String, failLabel: String) {
        val color = if (ok) R.color.status_connected else R.color.status_disconnected
        indicator.setBackgroundColor(ContextCompat.getColor(this, color))
        statusText.text = if (ok) okLabel else failLabel
    }

    private fun isMockLocationEnabled(): Boolean {
        return try {
            val opsManager = getSystemService(APP_OPS_SERVICE) as AppOpsManager
            val mode = opsManager.checkOpNoThrow(
                AppOpsManager.OPSTR_MOCK_LOCATION,
                android.os.Process.myUid(),
                packageName
            )
            mode == AppOpsManager.MODE_ALLOWED
        } catch (e: Exception) {
            false
        }
    }

    private fun observeViewModel() {
        viewModel.connectionStatus.observe(this) { status ->
            binding.textConnectionStatus.text = status
            val colorRes = when {
                status.contains("Connected", ignoreCase = true) ||
                    status.contains("Simulating", ignoreCase = true) -> R.color.status_connected
                status.contains("Connecting", ignoreCase = true) -> R.color.status_connecting
                else -> R.color.status_disconnected
            }
            binding.statusIndicator.setBackgroundColor(
                ContextCompat.getColor(this, colorRes)
            )
        }

        viewModel.isConnected.observe(this) { connected ->
            if (connected && !isServiceRunning) {
                isServiceRunning = true
                binding.buttonConnect.text = getString(R.string.disconnect)
            }
        }

        viewModel.simulationId.observe(this) { simId ->
            binding.textSimulationId.text = simId ?: getString(R.string.no_simulation)
        }

        viewModel.currentLat.observe(this) { lat ->
            updateLocationDisplay()
        }

        viewModel.currentLon.observe(this) { lon ->
            updateLocationDisplay()
        }

        viewModel.currentSpeed.observe(this) { speed ->
            val kmh = speed * 3.6f
            binding.textSpeed.text = String.format("%.1f km/h", kmh)
        }

        viewModel.currentBearing.observe(this) { bearing ->
            binding.textBearing.text = String.format("%.1f\u00B0", bearing)
        }
    }

    private fun updateLocationDisplay() {
        val lat = viewModel.currentLat.value ?: 0.0
        val lon = viewModel.currentLon.value ?: 0.0
        binding.textLocation.text = String.format("%.6f, %.6f", lat, lon)
    }

    private fun autoConnectIfReady() {
        if (isServiceRunning) return

        val locationGranted = ContextCompat.checkSelfPermission(
            this, Manifest.permission.ACCESS_FINE_LOCATION
        ) == PackageManager.PERMISSION_GRANTED

        if (!locationGranted) return
        if (!isMockLocationEnabled()) return

        // Auto-connect if we have a saved server config
        val host = prefs.getString("server_host", null)
        if (host.isNullOrBlank()) return

        startLocationService()
    }

    private fun saveServerConfig() {
        val host = binding.inputHost.text.toString().trim()
        val portStr = binding.inputPort.text.toString().trim()
        prefs.edit()
            .putString("server_host", host)
            .putString("server_port", portStr)
            .apply()
    }

    private fun startLocationService() {
        saveServerConfig()

        val host = binding.inputHost.text.toString().trim()
        val portStr = binding.inputPort.text.toString().trim()
        val port = portStr.toIntOrNull() ?: 50051
        val agentId = binding.textAgentId.text.toString()

        val intent = Intent(this, LocationService::class.java).apply {
            action = LocationService.ACTION_START
            putExtra(LocationService.EXTRA_HOST, host)
            putExtra(LocationService.EXTRA_PORT, port)
            putExtra(LocationService.EXTRA_AGENT_ID, agentId)
        }

        ContextCompat.startForegroundService(this, intent)
        isServiceRunning = true
        binding.buttonConnect.text = getString(R.string.disconnect)
    }

    private fun stopLocationService() {
        val intent = Intent(this, LocationService::class.java).apply {
            action = LocationService.ACTION_STOP
        }
        startService(intent)
        isServiceRunning = false
        binding.buttonConnect.text = getString(R.string.connect)
    }

    private fun requestPermissions() {
        val permissions = mutableListOf(
            Manifest.permission.ACCESS_FINE_LOCATION,
            Manifest.permission.ACCESS_COARSE_LOCATION
        )
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            permissions.add(Manifest.permission.POST_NOTIFICATIONS)
        }

        val needed = permissions.filter {
            ContextCompat.checkSelfPermission(this, it) != PackageManager.PERMISSION_GRANTED
        }

        if (needed.isNotEmpty()) {
            permissionLauncher.launch(needed.toTypedArray())
        }
    }

    override fun onDestroy() {
        MainViewModel.shared = null
        super.onDestroy()
    }
}
