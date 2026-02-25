/*
 * build.gradle.kts  (root project)
 *
 * Top-level build file for the Cheapskate Android project.
 *
 * This file declares the plugins needed by sub-projects but does NOT apply them here
 * (apply false).  Each sub-project applies the plugins it actually needs in its own
 * build.gradle.kts.
 *
 * Versions:
 *   AGP  8.3.x  – Android Gradle Plugin
 *   Kotlin 1.9.x – matches the Kotlin stdlib / coroutines versions used in :app
 */

plugins {
    // Android Application plugin – used by the :app module.
    id("com.android.application") version "8.3.2" apply false

    // Kotlin Android plugin – used by the :app module.
    id("org.jetbrains.kotlin.android") version "1.9.23" apply false
}
