package com.cheapskate.app

import android.content.Context
import android.util.Log
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext
import java.io.File
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL

/**
 * ServerManager handles the lifecycle of the embedded Go server binary.
 *
 * Responsibilities:
 *  - Extracting the arm64 binary from assets to the app's private files directory
 *  - Starting the Go server as a subprocess
 *  - Stopping the server subprocess on demand
 *  - Health-checking the server until it is ready to accept connections
 */
class ServerManager(private val context: Context) {

    companion object {
        private const val TAG = "ServerManager"

        /** Asset name of the pre-compiled Go binary. */
        private const val ASSET_BINARY_NAME = "server-arm64"

        /** Port the local server listens on. */
        const val SERVER_PORT = 8080

        /** Maximum number of health-check attempts before giving up. */
        private const val HEALTH_CHECK_RETRIES = 30

        /** Delay between health-check attempts in milliseconds. */
        private const val HEALTH_CHECK_DELAY_MS = 500L
    }

    /** Reference to the running server process, null when stopped. */
    private var serverProcess: Process? = null

    // ------------------------------------------------------------------
    // Public API
    // ------------------------------------------------------------------

    /**
     * Returns the base URL of the local server, e.g. "http://localhost:8080".
     */
    val localUrl: String get() = "http://localhost:$SERVER_PORT"

    /**
     * Returns true when [serverProcess] is alive.
     */
    val isRunning: Boolean
        get() = serverProcess?.isAlive == true

    /**
     * Full lifecycle start: extract binary → start process → wait for readiness.
     *
     * Must be called from a coroutine (uses [Dispatchers.IO] internally).
     *
     * @throws ServerStartException when the server cannot be started or does not become
     *         healthy within the retry window.
     */
    suspend fun startServer() = withContext(Dispatchers.IO) {
        if (isRunning) {
            Log.i(TAG, "Server already running, skipping start.")
            return@withContext
        }

        val binary = extractBinaryIfNeeded()
        launchProcess(binary)
        waitUntilReady()
    }

    /**
     * Stop the running server subprocess.
     * Safe to call when no process is running.
     */
    fun stopServer() {
        serverProcess?.let { process ->
            Log.i(TAG, "Stopping server process.")
            process.destroy()
            // Give it a moment to terminate cleanly before forcing.
            try {
                process.waitFor()
            } catch (e: InterruptedException) {
                Thread.currentThread().interrupt()
            }
        }
        serverProcess = null
        Log.i(TAG, "Server stopped.")
    }

    // ------------------------------------------------------------------
    // Binary extraction
    // ------------------------------------------------------------------

    /**
     * Copies [ASSET_BINARY_NAME] from assets into [Context.getFilesDir] if it has not
     * been copied yet (or if the existing copy is outdated / zero-length).
     *
     * Sets the executable bit after copying.
     *
     * @return [File] pointing to the extracted binary.
     */
    private fun extractBinaryIfNeeded(): File {
        val targetFile = File(context.filesDir, ASSET_BINARY_NAME)

        if (targetFile.exists() && targetFile.length() > 0) {
            Log.i(TAG, "Binary already extracted at ${targetFile.absolutePath}, skipping.")
            return targetFile
        }

        Log.i(TAG, "Extracting binary from assets to ${targetFile.absolutePath}…")
        try {
            context.assets.open(ASSET_BINARY_NAME).use { input ->
                targetFile.outputStream().use { output ->
                    input.copyTo(output)
                }
            }
        } catch (e: IOException) {
            throw ServerStartException(
                "Failed to extract server binary from assets. " +
                "Make sure '$ASSET_BINARY_NAME' exists in android/app/src/main/assets/.",
                e
            )
        }

        // Mark executable so the OS can run it.
        if (!targetFile.setExecutable(true, true)) {
            throw ServerStartException("Could not set executable bit on ${targetFile.absolutePath}.")
        }

        Log.i(TAG, "Binary extracted and made executable.")
        return targetFile
    }

    // ------------------------------------------------------------------
    // Process management
    // ------------------------------------------------------------------

    /**
     * Build and start the server subprocess.
     *
     * The command line mirrors the recommendation in the project README:
     *   ./server-arm64 --port 8080 --db <filesDir>/cheapskate.db --backup-path <filesDir>/backups
     */
    private fun launchProcess(binary: File) {
        val filesDir = context.filesDir.absolutePath
        val dbPath = "$filesDir/cheapskate.db"
        val backupPath = "$filesDir/backups"

        // Ensure the backups directory exists so the server does not fail on startup.
        File(backupPath).mkdirs()

        val command = listOf(
            binary.absolutePath,
            "--port", SERVER_PORT.toString(),
            "--db", dbPath,
            "--backup-path", backupPath
        )

        Log.i(TAG, "Launching: ${command.joinToString(" ")}")

        serverProcess = ProcessBuilder(command)
            .redirectErrorStream(true)          // merge stderr into stdout
            .directory(context.filesDir)        // working directory = private files
            .start()
            .also { process ->
                // Stream server logs to Logcat in a daemon thread.
                Thread({
                    process.inputStream.bufferedReader().forEachLine { line ->
                        Log.d(TAG, "[server] $line")
                    }
                }, "server-log-reader").apply {
                    isDaemon = true
                    start()
                }
            }

        Log.i(TAG, "Server process started (pid not available on API < 26).")
    }

    // ------------------------------------------------------------------
    // Health checking
    // ------------------------------------------------------------------

    /**
     * Polls the server's root endpoint until a 200-level response is received or the
     * retry limit is exhausted.
     *
     * @throws ServerStartException if the server does not become ready in time.
     */
    private suspend fun waitUntilReady() = withContext(Dispatchers.IO) {
        Log.i(TAG, "Waiting for server to become ready…")
        repeat(HEALTH_CHECK_RETRIES) { attempt ->
            if (!isRunning) {
                throw ServerStartException("Server process died during startup.")
            }
            if (isServerReachable()) {
                Log.i(TAG, "Server is ready after ${attempt + 1} attempt(s).")
                return@withContext
            }
            delay(HEALTH_CHECK_DELAY_MS)
        }
        throw ServerStartException(
            "Server did not become ready after " +
            "${HEALTH_CHECK_RETRIES * HEALTH_CHECK_DELAY_MS / 1000} seconds."
        )
    }

    /**
     * Performs a single HTTP GET to [localUrl] and returns true on any 2xx response.
     * Connection errors are swallowed and treated as "not yet ready".
     */
    private fun isServerReachable(): Boolean {
        return try {
            val conn = URL(localUrl).openConnection() as HttpURLConnection
            conn.connectTimeout = 300
            conn.readTimeout = 300
            val code = conn.responseCode
            conn.disconnect()
            code in 200..299
        } catch (_: Exception) {
            false
        }
    }

    // ------------------------------------------------------------------
    // Custom exception
    // ------------------------------------------------------------------

    /** Thrown when the server cannot be started for any reason. */
    class ServerStartException(message: String, cause: Throwable? = null) :
        Exception(message, cause)
}
