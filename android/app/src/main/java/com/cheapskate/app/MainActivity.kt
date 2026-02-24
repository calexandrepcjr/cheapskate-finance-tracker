package com.cheapskate.app

import android.annotation.SuppressLint
import android.content.Intent
import android.content.SharedPreferences
import android.net.Uri
import android.os.Bundle
import android.util.Log
import android.view.Menu
import android.view.MenuItem
import android.view.View
import android.webkit.WebChromeClient
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * MainActivity hosts the full-screen WebView that renders the Cheapskate finance tracker.
 *
 * Two modes are supported:
 *  1. **Local mode** – the embedded Go server binary is extracted and run as a subprocess;
 *     the WebView points to http://localhost:8080.
 *  2. **Remote mode** – the user provides an external server URL via the Settings dialog;
 *     no subprocess is started.
 *
 * Lifecycle:
 *  - onStart  → start local server (if in local mode)
 *  - onStop   → stop local server (if in local mode)
 *  - onBackPressed → navigate WebView back-stack before finishing the activity
 */
class MainActivity : AppCompatActivity() {

    companion object {
        private const val TAG = "MainActivity"

        // SharedPreferences keys
        private const val PREFS_NAME = "cheapskate_prefs"
        private const val PREF_REMOTE_MODE = "remote_mode"
        private const val PREF_REMOTE_URL = "remote_url"

        // Default placeholder for remote URL preference
        private const val DEFAULT_REMOTE_URL = "http://192.168.1.100:8080"
    }

    // ------------------------------------------------------------------
    // Views (set after setContentView)
    // ------------------------------------------------------------------

    private lateinit var webView: WebView
    private lateinit var loadingOverlay: LinearLayout
    private lateinit var loadingStatusText: TextView
    private lateinit var progressBar: ProgressBar

    // ------------------------------------------------------------------
    // State
    // ------------------------------------------------------------------

    private lateinit var prefs: SharedPreferences
    private lateinit var serverManager: ServerManager

    /** True when using an external server URL instead of the local subprocess. */
    private val isRemoteMode: Boolean
        get() = prefs.getBoolean(PREF_REMOTE_MODE, false)

    /** The URL the WebView loads: either localhost or the user-supplied remote URL. */
    private val activeUrl: String
        get() = if (isRemoteMode) {
            prefs.getString(PREF_REMOTE_URL, DEFAULT_REMOTE_URL) ?: DEFAULT_REMOTE_URL
        } else {
            serverManager.localUrl
        }

    // ------------------------------------------------------------------
    // Activity lifecycle
    // ------------------------------------------------------------------

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        prefs = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
        serverManager = ServerManager(this)

        bindViews()
        configureWebView()
    }

    override fun onStart() {
        super.onStart()
        if (!isRemoteMode) {
            startLocalServer()
        } else {
            // Remote mode: load immediately, no server management needed.
            hideLoadingOverlay()
            loadUrl(activeUrl)
        }
    }

    override fun onStop() {
        super.onStop()
        if (!isRemoteMode) {
            serverManager.stopServer()
        }
    }

    override fun onDestroy() {
        // Release WebView resources to avoid memory leaks.
        webView.stopLoading()
        webView.destroy()
        super.onDestroy()
    }

    // Handle hardware back button: navigate within WebView before closing the app.
    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        if (webView.canGoBack()) {
            webView.goBack()
        } else {
            @Suppress("DEPRECATION")
            super.onBackPressed()
        }
    }

    // ------------------------------------------------------------------
    // Options menu (Settings)
    // ------------------------------------------------------------------

    override fun onCreateOptionsMenu(menu: Menu): Boolean {
        menu.add(Menu.NONE, R.id.menu_settings, Menu.NONE, "Settings")
            .setShowAsAction(MenuItem.SHOW_AS_ACTION_NEVER)
        menu.add(Menu.NONE, R.id.menu_reload, Menu.NONE, "Reload")
            .setShowAsAction(MenuItem.SHOW_AS_ACTION_NEVER)
        return true
    }

    override fun onOptionsItemSelected(item: MenuItem): Boolean {
        return when (item.itemId) {
            R.id.menu_settings -> {
                showSettingsDialog()
                true
            }
            R.id.menu_reload -> {
                webView.reload()
                true
            }
            else -> super.onOptionsItemSelected(item)
        }
    }

    // ------------------------------------------------------------------
    // Settings dialog
    // ------------------------------------------------------------------

    /**
     * Shows an AlertDialog that lets the user toggle between local and remote mode.
     *
     * In remote mode the user can enter a custom server URL.
     * Changes take effect immediately (the server is restarted / stopped as needed).
     */
    private fun showSettingsDialog() {
        val dialogView = layoutInflater.inflate(R.layout.dialog_settings, null)
        val remoteUrlInput = dialogView.findViewById<EditText>(R.id.edit_remote_url)
        val remoteModeSwitch = dialogView.findViewById<android.widget.Switch>(R.id.switch_remote_mode)
        val remoteUrlContainer = dialogView.findViewById<View>(R.id.container_remote_url)

        // Populate current values.
        remoteModeSwitch.isChecked = isRemoteMode
        remoteUrlInput.setText(
            prefs.getString(PREF_REMOTE_URL, DEFAULT_REMOTE_URL)
        )
        remoteUrlContainer.visibility = if (isRemoteMode) View.VISIBLE else View.GONE

        remoteModeSwitch.setOnCheckedChangeListener { _, checked ->
            remoteUrlContainer.visibility = if (checked) View.VISIBLE else View.GONE
        }

        AlertDialog.Builder(this)
            .setTitle("Server Settings")
            .setView(dialogView)
            .setPositiveButton("Save") { _, _ ->
                val wasRemote = isRemoteMode
                val nowRemote = remoteModeSwitch.isChecked
                val newUrl = remoteUrlInput.text.toString().trimEnd('/')

                prefs.edit()
                    .putBoolean(PREF_REMOTE_MODE, nowRemote)
                    .putString(PREF_REMOTE_URL, newUrl.ifBlank { DEFAULT_REMOTE_URL })
                    .apply()

                applyModeChange(wasRemote = wasRemote, nowRemote = nowRemote)
            }
            .setNegativeButton("Cancel", null)
            .show()
    }

    /**
     * Applies a mode change after the Settings dialog is saved.
     *
     * - local → local: no change (same mode)
     * - local → remote: stop local server, load remote URL
     * - remote → local: start local server
     * - remote → remote: reload with (possibly new) remote URL
     */
    private fun applyModeChange(wasRemote: Boolean, nowRemote: Boolean) {
        when {
            !wasRemote && nowRemote -> {
                // Switching to remote: stop local server and navigate.
                serverManager.stopServer()
                hideLoadingOverlay()
                loadUrl(activeUrl)
            }
            wasRemote && !nowRemote -> {
                // Switching to local: start the server.
                startLocalServer()
            }
            else -> {
                // Same mode, just reload (URL may have changed in remote mode).
                loadUrl(activeUrl)
            }
        }
    }

    // ------------------------------------------------------------------
    // Server startup
    // ------------------------------------------------------------------

    /**
     * Launches the embedded Go server in a coroutine and shows a loading screen.
     * On success the WebView is pointed at localhost.
     * On failure an error dialog is shown with the option to retry.
     */
    private fun startLocalServer() {
        showLoadingOverlay("Starting local server…")

        lifecycleScope.launch {
            try {
                serverManager.startServer()
                withContext(Dispatchers.Main) {
                    hideLoadingOverlay()
                    loadUrl(serverManager.localUrl)
                }
            } catch (e: ServerManager.ServerStartException) {
                Log.e(TAG, "Failed to start server", e)
                withContext(Dispatchers.Main) {
                    showServerError(e.message ?: "Unknown error starting server.")
                }
            }
        }
    }

    // ------------------------------------------------------------------
    // View setup
    // ------------------------------------------------------------------

    private fun bindViews() {
        webView = findViewById(R.id.web_view)
        loadingOverlay = findViewById(R.id.loading_overlay)
        loadingStatusText = findViewById(R.id.text_loading_status)
        progressBar = findViewById(R.id.progress_bar)
    }

    /**
     * Configures the WebView for HTMX-powered server-side rendering.
     *
     * Key settings:
     *  - JavaScript enabled (required for HTMX)
     *  - DOM storage enabled
     *  - Custom [WebViewClient] to keep navigation in-app
     *  - Custom [WebChromeClient] to mirror page load progress on the ProgressBar
     */
    @SuppressLint("SetJavaScriptEnabled")
    private fun configureWebView() {
        webView.settings.apply {
            javaScriptEnabled = true          // HTMX requires JS
            domStorageEnabled = true          // localStorage / sessionStorage
            cacheMode = WebSettings.LOAD_DEFAULT
            setSupportZoom(false)
            builtInZoomControls = false
            displayZoomControls = false
            useWideViewPort = true
            loadWithOverviewMode = true
        }

        webView.webViewClient = object : WebViewClient() {
            /**
             * Keep all navigation inside the WebView.
             * External links (non-localhost) open in the system browser.
             */
            override fun shouldOverrideUrlLoading(
                view: WebView,
                request: WebResourceRequest
            ): Boolean {
                val host = request.url.host ?: return false
                return if (host == "localhost" || host == "127.0.0.1") {
                    false // Let WebView handle it.
                } else {
                    // Open external URLs in the default browser.
                    startActivity(Intent(Intent.ACTION_VIEW, request.url))
                    true
                }
            }

            override fun onReceivedError(
                view: WebView,
                request: WebResourceRequest,
                error: WebResourceError
            ) {
                // Only handle errors for the main frame (not sub-resources).
                if (request.isForMainFrame) {
                    Log.w(TAG, "WebView error: ${error.errorCode} – ${error.description}")
                }
            }
        }

        webView.webChromeClient = object : WebChromeClient() {
            override fun onProgressChanged(view: WebView, newProgress: Int) {
                progressBar.progress = newProgress
                progressBar.visibility = if (newProgress < 100) View.VISIBLE else View.GONE
            }
        }
    }

    private fun loadUrl(url: String) {
        Log.i(TAG, "Loading URL: $url")
        webView.loadUrl(url)
    }

    // ------------------------------------------------------------------
    // Loading overlay helpers
    // ------------------------------------------------------------------

    private fun showLoadingOverlay(message: String) {
        loadingStatusText.text = message
        loadingOverlay.visibility = View.VISIBLE
        webView.visibility = View.GONE
    }

    private fun hideLoadingOverlay() {
        loadingOverlay.visibility = View.GONE
        webView.visibility = View.VISIBLE
    }

    // ------------------------------------------------------------------
    // Error handling
    // ------------------------------------------------------------------

    private fun showServerError(message: String) {
        AlertDialog.Builder(this)
            .setTitle("Server Error")
            .setMessage("Could not start the local server:\n\n$message")
            .setPositiveButton("Retry") { _, _ -> startLocalServer() }
            .setNegativeButton("Switch to Remote") { _, _ ->
                prefs.edit().putBoolean(PREF_REMOTE_MODE, true).apply()
                showSettingsDialog()
            }
            .setCancelable(false)
            .show()
    }
}
