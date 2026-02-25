# proguard-rules.pro
#
# Project-specific ProGuard / R8 rules for the Cheapskate app.
#
# This is a prototype app with a minimal dependency set.  The rules below
# cover the libraries used; add more as new dependencies are introduced.

# ── AndroidX / AppCompat ───────────────────────────────────────────────────────
-keep class androidx.appcompat.** { *; }
-dontwarn androidx.appcompat.**

# ── Kotlin coroutines ─────────────────────────────────────────────────────────
# Preserve coroutine metadata used by the debugger and stack-trace recovery.
-keepnames class kotlinx.coroutines.internal.MainDispatcherFactory {}
-keepnames class kotlinx.coroutines.CoroutineExceptionHandler {}
-keepclassmembernames class kotlinx.coroutines.** {
    volatile <fields>;
}

# ── Keep application classes ──────────────────────────────────────────────────
-keep class com.cheapskate.app.** { *; }

# ── General Android rules ─────────────────────────────────────────────────────
-keepattributes SourceFile,LineNumberTable
-renamesourcefileattribute SourceFile
