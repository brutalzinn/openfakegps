package com.openfakegps.agent

import android.app.Application
import android.util.Log
import com.openfakegps.agent.location.MockLocationProvider

class App : Application() {

    var mockLocationProvider: MockLocationProvider? = null
        private set

    override fun onCreate() {
        super.onCreate()
        instance = this

        // Register the test provider early so the system recognizes this app
        // as the active mock location provider in Developer Options.
        mockLocationProvider = MockLocationProvider(this).also {
            if (it.initialize()) {
                Log.i(TAG, "Mock location provider registered on app start")
            }
        }
    }

    companion object {
        private const val TAG = "App"
        lateinit var instance: App
            private set
    }
}
