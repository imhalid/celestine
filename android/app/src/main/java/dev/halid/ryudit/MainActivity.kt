package dev.halid.ryudit

import android.annotation.SuppressLint
import android.app.Activity
import android.content.Intent
import android.graphics.Color
import android.os.Bundle
import android.view.View
import android.webkit.MimeTypeMap
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient

/**
 * ryudit on Android: the web build (assets/web: index.html, the wasm, wasm_exec.js) in a WebView. The
 * files are served from the assets under a made-up https origin, so that the wasm loads with its own
 * MIME type and the page has an origin for localStorage, where the library is kept (store_js.go).
 * A text shared to the app ("Ryudit'e ekle") is put on the page's address as #share=<text>.
 */
class MainActivity : Activity() {
    private lateinit var web: WebView

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        web = WebView(this)
        web.setBackgroundColor(Color.parseColor("#17171a"))
        setContentView(web)
        web.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            mediaPlaybackRequiresUserGesture = false
            cacheMode = WebSettings.LOAD_DEFAULT
            allowFileAccess = false
        }
        web.webChromeClient = WebChromeClient()
        web.webViewClient = object : WebViewClient() {
            override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? {
                val url = request.url
                if (url.host != HOST) return null
                var path = url.path ?: "/"
                if (path == "/" || path.isEmpty()) path = "/index.html"
                return try {
                    val stream = assets.open("web$path")
                    val ext = MimeTypeMap.getFileExtensionFromUrl(path)
                    val mime = when (ext) {
                        "wasm" -> "application/wasm"
                        "js" -> "text/javascript"
                        "html" -> "text/html"
                        "css" -> "text/css"
                        "json", "webmanifest" -> "application/json"
                        else -> MimeTypeMap.getSingleton().getMimeTypeFromExtension(ext) ?: "application/octet-stream"
                    }
                    WebResourceResponse(mime, "utf-8", stream)
                } catch (e: Exception) {
                    WebResourceResponse("text/plain", "utf-8", 404, "Not Found", emptyMap(), "".byteInputStream())
                }
            }
        }
        web.systemUiVisibility = View.SYSTEM_UI_FLAG_LAYOUT_STABLE
        var start = "https://$HOST/index.html"
        val shared = intent?.takeIf { it.action == Intent.ACTION_SEND }?.getStringExtra(Intent.EXTRA_TEXT)
        if (shared != null) start += "#share=" + android.net.Uri.encode(shared)
        web.loadUrl(start)
    }

    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        // back goes through the app's own history first (⌘[ ), then leaves
        web.evaluateJavascript("window.dispatchEvent(new KeyboardEvent('keydown',{key:'[',code:'BracketLeft',metaKey:true,ctrlKey:true,bubbles:true}))", null)
        @Suppress("DEPRECATION") super.onBackPressed()
    }

    companion object { const val HOST = "appassets.ryudit.local" }
}
