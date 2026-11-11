plugins { id("com.android.application"); id("org.jetbrains.kotlin.android") }
android {
    namespace = "dev.halid.celestine"
    compileSdk = 36
    defaultConfig { applicationId = "dev.halid.celestine"; minSdk = 26; targetSdk = 36; versionCode = 1; versionName = "0.1-web" }
    compileOptions { sourceCompatibility = JavaVersion.VERSION_17; targetCompatibility = JavaVersion.VERSION_17 }
    androidResources { noCompress += listOf("wasm") }
}
kotlin { compilerOptions { jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17) } }
