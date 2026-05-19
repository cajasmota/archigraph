package engine

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Ktor HttpClient
// ---------------------------------------------------------------------------

// TestKtClient_KtorLiteral covers `httpClient.get("/users")` and
// `HttpClient().post("/users")` call forms.
func TestKtClient_KtorLiteral(t *testing.T) {
	src := `
import io.ktor.client.*
import io.ktor.client.request.*

suspend fun fetchUsers(): String {
    val httpClient = HttpClient()
    return httpClient.get("/api/users")
}

suspend fun createUser(body: String): String {
    val httpClient = HttpClient()
    return httpClient.post("/api/users")
}
`
	ids, rels := runDetectWithRels(t, "kotlin", "UsersClient.kt", src)
	want := []string{
		"http:GET:/api/users",
		"http:POST:/api/users",
	}
	requireContains(t, ids, want, "kt-ktor-literal")
	requireFetches(t, rels, "http:GET:/api/users", "kt-ktor-literal")
	requireFetches(t, rels, "http:POST:/api/users", "kt-ktor-literal")
}

// TestKtClient_KtorInlineConstruction covers `HttpClient().get(url)` with
// the inline HttpClient() construction pattern.
func TestKtClient_KtorInlineConstruction(t *testing.T) {
	src := `
import io.ktor.client.*
import io.ktor.client.request.*

suspend fun getOrders(): String {
    return HttpClient().get("/api/orders")
}
`
	ids, rels := runDetectWithRels(t, "kotlin", "InlineClient.kt", src)
	requireContains(t, ids, []string{"http:GET:/api/orders"}, "kt-ktor-inline")
	requireFetches(t, rels, "http:GET:/api/orders", "kt-ktor-inline")
}

// TestKtClient_KtorRequestBuilder covers `httpClient.request { url("...") }`.
func TestKtClient_KtorRequestBuilder(t *testing.T) {
	src := `
import io.ktor.client.*
import io.ktor.client.request.*

suspend fun fetchHealth() {
    httpClient.request {
        url("https://example.com/api/health")
    }
}
`
	ids, rels := runDetectWithRels(t, "kotlin", "RequestBuilderClient.kt", src)
	requireContains(t, ids, []string{"http:GET:/api/health"}, "kt-ktor-request-builder")
	requireFetches(t, rels, "http:GET:/api/health", "kt-ktor-request-builder")
}

// TestKtClient_KtorUseLambda covers the coroutine form
// `httpClient.use { it.get("/path") }`.
func TestKtClient_KtorUseLambda(t *testing.T) {
	src := `
import io.ktor.client.*
import io.ktor.client.request.*

suspend fun callApi(): String {
    return httpClient.use { it.get("/api/products") }
}
`
	ids, rels := runDetectWithRels(t, "kotlin", "CoroutineClient.kt", src)
	requireContains(t, ids, []string{"http:GET:/api/products"}, "kt-ktor-use-lambda")
	requireFetches(t, rels, "http:GET:/api/products", "kt-ktor-use-lambda")
}

// ---------------------------------------------------------------------------
// OkHttp (Kotlin)
// ---------------------------------------------------------------------------

// TestKtClient_OkHttpRequestBuilder covers
// `OkHttpClient().newCall(Request.Builder().url("...").build())`.
func TestKtClient_OkHttpRequestBuilder(t *testing.T) {
	src := `
import okhttp3.*

fun fetchProfile(): Response {
    val request = Request.Builder()
        .url("https://api.example.com/api/profile")
        .build()
    return OkHttpClient().newCall(request).execute()
}
`
	ids, rels := runDetectWithRels(t, "kotlin", "OkHttpClient.kt", src)
	requireContains(t, ids, []string{"http:GET:/api/profile"}, "kt-okhttp-builder")
	requireFetches(t, rels, "http:GET:/api/profile", "kt-okhttp-builder")
}

// TestKtClient_OkHttpRequestBuilderPost covers
// `Request.Builder().url("...").post(body).build()`.
func TestKtClient_OkHttpRequestBuilderPost(t *testing.T) {
	src := `
import okhttp3.*

fun submitData(body: RequestBody): Response {
    val request = Request.Builder()
        .url("/api/submissions")
        .post(body)
        .build()
    return OkHttpClient().newCall(request).execute()
}
`
	ids, rels := runDetectWithRels(t, "kotlin", "OkHttpPost.kt", src)
	requireContains(t, ids, []string{"http:POST:/api/submissions"}, "kt-okhttp-post")
	requireFetches(t, rels, "http:POST:/api/submissions", "kt-okhttp-post")
}

// ---------------------------------------------------------------------------
// Retrofit (Kotlin)
// ---------------------------------------------------------------------------

// TestKtClient_RetrofitAnnotation covers `@GET("/api/users")` on an
// interface suspend fun.
func TestKtClient_RetrofitAnnotation(t *testing.T) {
	src := `
import retrofit2.http.*
import retrofit2.*

interface UserApi {
    @GET("/api/users")
    suspend fun listUsers(): List<User>

    @POST("/api/users")
    suspend fun createUser(@Body user: User): User

    @DELETE("/api/users/{id}")
    suspend fun deleteUser(@Path("id") id: Long)
}
`
	ids, rels := runDetectWithRels(t, "kotlin", "UserApi.kt", src)
	want := []string{
		"http:GET:/api/users",
		"http:POST:/api/users",
		"http:DELETE:/api/users/{id}",
	}
	requireContains(t, ids, want, "kt-retrofit-annotation")
	requireFetches(t, rels, "http:GET:/api/users", "kt-retrofit-annotation")
	requireFetches(t, rels, "http:POST:/api/users", "kt-retrofit-annotation")
}

// TestKtClient_RetrofitBaseURL covers Retrofit.Builder().baseUrl("...")
// composed with @GET annotation paths.
func TestKtClient_RetrofitBaseURL(t *testing.T) {
	src := `
import retrofit2.*
import retrofit2.http.*

interface SearchApi {
    @GET("/search")
    suspend fun search(@Query("q") query: String): SearchResult
}

val retrofit = Retrofit.Builder()
    .baseUrl("https://api.example.com")
    .build()
`
	ids, rels := runDetectWithRels(t, "kotlin", "SearchApi.kt", src)
	requireContains(t, ids, []string{"http:GET:/search"}, "kt-retrofit-baseurl")
	requireFetches(t, rels, "http:GET:/search", "kt-retrofit-baseurl")
}

// ---------------------------------------------------------------------------
// Env-var concatenation
// ---------------------------------------------------------------------------

// TestKtClient_EnvVarConcat covers
// `client.get(System.getenv("API_URL") + "/users")` → runtime_dynamic=true.
func TestKtClient_EnvVarConcat(t *testing.T) {
	src := `
import io.ktor.client.*
import io.ktor.client.request.*

suspend fun callRemote(): String {
    val httpClient = HttpClient()
    return httpClient.get(System.getenv("API_URL") + "/users")
}
`
	ids, rels := runDetectWithRels(t, "kotlin", "EnvClient.kt", src)
	requireContains(t, ids, []string{"http:GET:/users"}, "kt-env-var-concat")
	requireFetches(t, rels, "http:GET:/users", "kt-env-var-concat")

	// Verify runtime_dynamic=true.
	_, res := runDetect(t, "kotlin", "EnvClient.kt", src)
	found := false
	for _, e := range res.Entities {
		if e.ID == "http:GET:/users" && e.Properties["runtime_dynamic"] == "true" {
			found = true
		}
	}
	if !found {
		t.Errorf("kt-env-var-concat: expected runtime_dynamic=true on http:GET:/users")
	}
}

// ---------------------------------------------------------------------------
// Negative case
// ---------------------------------------------------------------------------

// TestKtClient_Negative verifies that a non-HTTP Kotlin file does not emit
// any http_endpoint synthetics.
func TestKtClient_Negative(t *testing.T) {
	src := `
package com.example

import java.util.ArrayList

fun processData(items: List<String>): List<String> {
    val result = ArrayList<String>()
    for (item in items) {
        result.add(item.uppercase())
    }
    return result
}

data class DataModel(val name: String, val value: Int)
`
	ids, _ := runDetectWithRels(t, "kotlin", "DataProcessor.kt", src)
	for _, id := range ids {
		if strings.HasPrefix(id, "http:") {
			t.Errorf("kt-negative: unexpected http_endpoint %q from non-HTTP file", id)
		}
	}
}

// TestKtClient_KtorVerbCoverage covers all HTTP verbs on Ktor.
func TestKtClient_KtorVerbCoverage(t *testing.T) {
	src := `
import io.ktor.client.*
import io.ktor.client.request.*

suspend fun doAllVerbs() {
    httpClient.get("/api/items")
    httpClient.post("/api/items")
    httpClient.put("/api/items/1")
    httpClient.patch("/api/items/1")
    httpClient.delete("/api/items/1")
    httpClient.head("/api/items")
    httpClient.options("/api/items")
}
`
	ids, _ := runDetectWithRels(t, "kotlin", "AllVerbs.kt", src)
	want := []string{
		"http:GET:/api/items",
		"http:POST:/api/items",
		"http:PUT:/api/items/1",
		"http:PATCH:/api/items/1",
		"http:DELETE:/api/items/1",
		"http:HEAD:/api/items",
		"http:OPTIONS:/api/items",
	}
	requireContains(t, ids, want, "kt-ktor-all-verbs")
}
