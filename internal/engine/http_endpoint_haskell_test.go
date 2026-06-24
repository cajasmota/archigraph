package engine

import "testing"

// TestScotty_BasicRoute covers the common Scotty verb shape:
// get "/todos" $ do ... / post "/todos" $ do ...
func TestScotty_BasicRoute(t *testing.T) {
	src := `
{-# LANGUAGE OverloadedStrings #-}
import Web.Scotty

main :: IO ()
main = scotty 3000 $ do
  get "/todos" $ do
    json allTodos
  post "/todos" $ do
    json newTodo
`
	ids, _ := runDetect(t, "haskell", "app/Main.hs", src)
	requireContains(t, ids, []string{
		"http:GET:/todos",
		"http:POST:/todos",
	}, "scotty-basic-route")
}

// TestScotty_PathParam covers the Sinatra-style `:param` dynamic segment:
// get "/todos/:id" → GET /todos/{id}.
func TestScotty_PathParam(t *testing.T) {
	src := `
import Web.Scotty

main = scotty 3000 $ do
  get "/todos/:id" $ do
    tid <- param "id"
    json (findTodo tid)
  delete "/todos/:id" $ text "deleted"
`
	ids, _ := runDetect(t, "haskell", "app/Routes.hs", src)
	requireContains(t, ids, []string{
		"http:GET:/todos/{id}",
		"http:DELETE:/todos/{id}",
	}, "scotty-path-param")
}

// TestScotty_NonWebFileIgnored is the negative guard: a Haskell file with no
// Scotty marker that happens to call a function named `get` must not synthesize
// an endpoint.
func TestScotty_NonWebFileIgnored(t *testing.T) {
	src := `
module Cache where

lookup' :: Store -> Key -> Maybe Value
lookup' store key = get store key
`
	ids, _ := runDetect(t, "haskell", "src/Cache.hs", src)
	for _, id := range ids {
		if id == "http:GET:/store" || id == "http:GET:/key" {
			t.Fatalf("non-web file must not synthesize an endpoint; got %v", ids)
		}
	}
}

// TestYesod_ParseRoutes covers the Yesod parseRoutes quasiquote: a literal
// route, a typed single-segment capture `#UserId` and multi-verb lines.
func TestYesod_ParseRoutes(t *testing.T) {
	src := `
{-# LANGUAGE QuasiQuotes #-}
import Yesod

mkYesod "App" [parseRoutes|
/users        UsersR  GET POST
/user/#UserId UserR   GET
/static/*Texts StaticR GET
|]
`
	ids, _ := runDetect(t, "haskell", "src/Foundation.hs", src)
	requireContains(t, ids, []string{
		"http:GET:/users",
		"http:POST:/users",
		"http:GET:/user/{userid}",
		"http:GET:/static/{texts}",
	}, "yesod-parse-routes")
}

// TestServant_TypeLevelAPI covers the Servant `:>`-chain type-level DSL with a
// Capture path param and a `:<|>` alternation.
func TestServant_TypeLevelAPI(t *testing.T) {
	src := `
{-# LANGUAGE DataKinds #-}
{-# LANGUAGE TypeOperators #-}
import Servant

type API = "users" :> Capture "id" Int :> Get '[JSON] User
      :<|> "users" :> ReqBody '[JSON] User :> Post '[JSON] User
`
	ids, _ := runDetect(t, "haskell", "src/Api.hs", src)
	requireContains(t, ids, []string{
		"http:GET:/users/{id}",
		"http:POST:/users",
	}, "servant-type-level-api")
}
