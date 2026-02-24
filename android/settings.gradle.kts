/*
 * settings.gradle.kts
 *
 * Gradle settings file for the Cheapskate Android project.
 * Declares the project name and the single included sub-project (the :app module).
 */

pluginManagement {
    repositories {
        // Gradle plugin portal (required for Kotlin and AGP plugins)
        gradlePluginPortal()
        // Google's Maven repository (required for Android Gradle Plugin)
        google()
        // Maven Central (fallback for any transitive plugin dependencies)
        mavenCentral()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "cheapskate"

// The only module in this project.
include(":app")
