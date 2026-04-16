package com.openfakegps.agent.ui

import androidx.lifecycle.LiveData
import androidx.lifecycle.MutableLiveData
import androidx.lifecycle.ViewModel

class MainViewModel : ViewModel() {

    private val _connectionStatus = MutableLiveData("Disconnected")
    val connectionStatus: LiveData<String> = _connectionStatus

    private val _isConnected = MutableLiveData(false)
    val isConnected: LiveData<Boolean> = _isConnected

    private val _simulationId = MutableLiveData<String?>(null)
    val simulationId: LiveData<String?> = _simulationId

    private val _currentLat = MutableLiveData(0.0)
    val currentLat: LiveData<Double> = _currentLat

    private val _currentLon = MutableLiveData(0.0)
    val currentLon: LiveData<Double> = _currentLon

    private val _currentSpeed = MutableLiveData(0f)
    val currentSpeed: LiveData<Float> = _currentSpeed

    private val _currentBearing = MutableLiveData(0f)
    val currentBearing: LiveData<Float> = _currentBearing

    fun updateConnectionStatus(status: String, connected: Boolean) {
        _connectionStatus.postValue(status)
        _isConnected.postValue(connected)
    }

    fun updateSimulationId(id: String?) {
        _simulationId.postValue(id)
    }

    fun updateLocation(lat: Double, lon: Double, speed: Float, bearing: Float) {
        _currentLat.postValue(lat)
        _currentLon.postValue(lon)
        _currentSpeed.postValue(speed)
        _currentBearing.postValue(bearing)
    }

    fun reset() {
        _connectionStatus.postValue("Disconnected")
        _isConnected.postValue(false)
        _simulationId.postValue(null)
        _currentLat.postValue(0.0)
        _currentLon.postValue(0.0)
        _currentSpeed.postValue(0f)
        _currentBearing.postValue(0f)
    }

    companion object {
        // Singleton-style access for service updates.
        // In production, consider a shared repository or bound service pattern.
        @Volatile
        var shared: MainViewModel? = null
    }
}
