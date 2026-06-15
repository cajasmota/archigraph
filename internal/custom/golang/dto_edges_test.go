package golang_test

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"

	_ "github.com/cajasmota/grafel/internal/custom/golang"
)

// dtoRecords runs the Go DTO extractor and returns the full entity records so
// the endpoint→DTO edge tests (#3629/#3607) can assert ACCEPTS_INPUT / RETURNS.
func dtoRecords(t *testing.T, src string) []types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get("custom_go_dto")
	if !ok {
		t.Fatal("custom_go_dto not registered")
	}
	ents, err := e.Extract(context.Background(), extreg.FileInput{
		Path: "handler.go", Language: "go", Content: []byte(src),
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return ents
}

func hasGoDTOEdge(ents []types.EntityRecord, kind, toID string) bool {
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == kind && r.ToID == toID {
				return true
			}
		}
	}
	return false
}

// gin c.ShouldBindJSON(&LoginReq{}) → ACCEPTS_INPUT LoginReq.
func TestGoDTOEdge_GinAcceptsInput(t *testing.T) {
	src := `package main

import "github.com/gin-gonic/gin"

type LoginReq struct {
	Email    string ` + "`json:\"email\"`" + `
	Password string ` + "`json:\"password\"`" + `
}

func login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}
}
`
	ents := dtoRecords(t, src)
	if !hasGoDTOEdge(ents, "ACCEPTS_INPUT", "Class:LoginReq") {
		t.Fatal("expected ACCEPTS_INPUT -> Class:LoginReq")
	}
}

// gin c.JSON(200, resp) where resp is a known struct → RETURNS that struct.
func TestGoDTOEdge_GinReturns(t *testing.T) {
	src := `package main

import "github.com/gin-gonic/gin"

type UserResp struct {
	ID   int    ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func getUser(c *gin.Context) {
	var resp UserResp
	c.JSON(200, resp)
}
`
	ents := dtoRecords(t, src)
	if !hasGoDTOEdge(ents, "RETURNS", "Class:UserResp") {
		t.Fatal("expected RETURNS -> Class:UserResp")
	}
}

// echo c.Bind(&CreateReq) → ACCEPTS_INPUT CreateReq.
func TestGoDTOEdge_EchoAcceptsInput(t *testing.T) {
	src := `package main

import "github.com/labstack/echo/v4"

type CreateReq struct {
	Title string ` + "`json:\"title\"`" + `
}

func create(c echo.Context) error {
	var req CreateReq
	if err := c.Bind(&req); err != nil {
		return err
	}
	return nil
}
`
	ents := dtoRecords(t, src)
	if !hasGoDTOEdge(ents, "ACCEPTS_INPUT", "Class:CreateReq") {
		t.Fatal("expected ACCEPTS_INPUT -> Class:CreateReq")
	}
}

// An unresolved bind target (no file-local struct) emits a DTO entity but NO
// edge — we never point an edge at an unknown type (honest-partial).
func TestGoDTOEdge_UnresolvedNoEdge(t *testing.T) {
	src := `package main

import "github.com/gin-gonic/gin"

func login(c *gin.Context) {
	var req someExternalType
	_ = c.ShouldBindJSON(&req)
}
`
	ents := dtoRecords(t, src)
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == "ACCEPTS_INPUT" || r.Kind == "RETURNS" {
				t.Errorf("expected no DTO edge for unresolved type, got %s -> %s", r.Kind, r.ToID)
			}
		}
	}
}

// A file with no recognised framework marker emits nothing — no edges leak.
func TestGoDTOEdge_NoFrameworkNoEdge(t *testing.T) {
	src := `package main

type Foo struct{ A int }

func main() { _ = Foo{} }
`
	ents := dtoRecords(t, src)
	if len(ents) != 0 {
		t.Fatalf("expected 0 entities for non-framework file, got %d", len(ents))
	}
}
