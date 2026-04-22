package com.openfakegps.agent.grpc

import android.util.Log
import com.openfakegps.proto.v1.AgentServiceGrpcKt
import com.openfakegps.proto.v1.agentMessage
import com.openfakegps.proto.v1.heartbeat
import com.openfakegps.proto.v1.registerRequest
import com.openfakegps.proto.v1.statusUpdate
import com.openfakegps.proto.v1.DeviceStatus
import com.openfakegps.proto.v1.ServerMessage
import com.openfakegps.proto.v1.AgentMessage
import io.grpc.ManagedChannel
import io.grpc.ManagedChannelBuilder
import io.grpc.StatusException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.flow.merge
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.onCompletion
import kotlinx.coroutines.flow.onEach
import kotlinx.coroutines.flow.onStart
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.util.concurrent.TimeUnit

interface AgentCallback {
    fun onConnected()
    fun onDisconnected()
    fun onLocationUpdate(update: LocationUpdateData)
    fun onSimulationCommand(action: String, simulationId: String)
    fun onError(error: String)
}

data class LocationUpdateData(
    val simulationId: String,
    val latitude: Double,
    val longitude: Double,
    val speed: Float,
    val bearing: Float,
    val accuracy: Float,
    val altitude: Double,
    val timestampMs: Long
)

class AgentClient(
    private val host: String,
    private val port: Int,
    private val agentId: String,
    private val deviceName: String,
    private val deviceModel: String,
    private val callback: AgentCallback
) {
    companion object {
        private const val TAG = "AgentClient"
        private const val HEARTBEAT_INTERVAL_MS = 10_000L
        private const val MAX_BACKOFF_MS = 30_000L
        private const val INITIAL_BACKOFF_MS = 1_000L
    }

    private var channel: ManagedChannel? = null
    private var scope: CoroutineScope? = null
    private var connectJob: Job? = null
    private var heartbeatJob: Job? = null

    private val outgoingMessages = MutableSharedFlow<AgentMessage>(extraBufferCapacity = 64)

    @Volatile
    private var isRunning = false

    fun connect() {
        if (isRunning) return
        isRunning = true

        scope = CoroutineScope(Dispatchers.IO + SupervisorJob())
        scope?.launch {
            var backoff = INITIAL_BACKOFF_MS
            while (isActive && isRunning) {
                try {
                    doConnect()
                    // If doConnect returns normally, connection ended cleanly
                    backoff = INITIAL_BACKOFF_MS
                } catch (e: Exception) {
                    Log.e(TAG, "Connection error: ${e.message}", e)
                    callback.onError(e.message ?: "Unknown error")
                }
                callback.onDisconnected()
                if (!isRunning) break
                Log.i(TAG, "Reconnecting in ${backoff}ms...")
                delay(backoff)
                backoff = (backoff * 2).coerceAtMost(MAX_BACKOFF_MS)
            }
        }
    }

    private suspend fun doConnect() {
        channel?.shutdownNow()
        channel = ManagedChannelBuilder
            .forAddress(host, port)
            .usePlaintext()
            .keepAliveWithoutCalls(true)
            .keepAliveTime(15, TimeUnit.SECONDS)
            .keepAliveTimeout(10, TimeUnit.SECONDS)
            .build()

        val stub = AgentServiceGrpcKt.AgentServiceCoroutineStub(channel!!)

        val registerMsg = agentMessage {
            register = registerRequest {
                this.agentId = this@AgentClient.agentId
                this.deviceName = this@AgentClient.deviceName
                this.deviceModel = this@AgentClient.deviceModel
                this.capabilities.addAll(listOf("mock_location", "gps_simulation"))
            }
        }

        // Start heartbeat
        heartbeatJob?.cancel()
        heartbeatJob = scope?.launch {
            while (isActive && isRunning) {
                delay(HEARTBEAT_INTERVAL_MS)
                val hb = agentMessage {
                    heartbeat = heartbeat {
                        timestampMs = System.currentTimeMillis()
                    }
                }
                outgoingMessages.emit(hb)
            }
        }

        // Use onStart to ensure register message is sent first when the flow is collected
        val requestFlow: Flow<AgentMessage> = outgoingMessages
            .onStart { emit(registerMsg) }

        try {
            val responseFlow = stub.connect(requestFlow)

            responseFlow
                .onEach { serverMessage -> handleServerMessage(serverMessage) }
                .catch { e ->
                    Log.e(TAG, "Stream error: ${e.message}", e)
                    callback.onError(e.message ?: "Stream error")
                }
                .onCompletion {
                    heartbeatJob?.cancel()
                }
                .collect()
        } catch (e: StatusException) {
            Log.e(TAG, "gRPC status error: ${e.status}", e)
            throw e
        } finally {
            heartbeatJob?.cancel()
            channel?.shutdown()?.awaitTermination(5, TimeUnit.SECONDS)
        }
    }

    private fun handleServerMessage(message: ServerMessage) {
        when {
            message.hasRegisterResponse() -> {
                val resp = message.registerResponse
                if (resp.accepted) {
                    Log.i(TAG, "Registration accepted: ${resp.message}")
                    callback.onConnected()
                } else {
                    Log.w(TAG, "Registration rejected: ${resp.message}")
                    callback.onError("Registration rejected: ${resp.message}")
                }
            }
            message.hasLocationUpdate() -> {
                val loc = message.locationUpdate
                val data = LocationUpdateData(
                    simulationId = loc.simulationId,
                    latitude = loc.latitude,
                    longitude = loc.longitude,
                    speed = loc.speed,
                    bearing = loc.bearing,
                    accuracy = loc.accuracy,
                    altitude = loc.altitude,
                    timestampMs = loc.timestampMs
                )
                callback.onLocationUpdate(data)
            }
            message.hasSimulationCommand() -> {
                val cmd = message.simulationCommand
                callback.onSimulationCommand(
                    action = cmd.action.name,
                    simulationId = cmd.simulationId
                )
            }
        }
    }

    suspend fun sendStatusUpdate(status: DeviceStatus, msg: String = "") {
        val update = agentMessage {
            statusUpdate = statusUpdate {
                this.agentId = this@AgentClient.agentId
                this.status = status
                this.message = msg
            }
        }
        outgoingMessages.emit(update)
    }

    fun disconnect() {
        isRunning = false
        heartbeatJob?.cancel()
        connectJob?.cancel()
        scope?.cancel()
        scope = null
        try {
            channel?.shutdownNow()?.awaitTermination(2, TimeUnit.SECONDS)
        } catch (e: Exception) {
            Log.w(TAG, "Error shutting down channel", e)
        }
        channel = null
    }
}
