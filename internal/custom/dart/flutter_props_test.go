package dart_test

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"

	_ "github.com/cajasmota/grafel/internal/custom/dart"
)

// TestFlutterWidgetProps proves a widget's `final <Type> <name>;` instance
// fields are extracted as SCOPE.Pattern/prop_extraction entities with a
// HAS_PROPS edge from the widget component (Data Flow/prop_extraction, #5361),
// and that a non-widget class's fields are NOT mis-tagged as props.
func TestFlutterWidgetProps(t *testing.T) {
	src := `
class UserCard extends StatelessWidget {
  final String title;
  final int count;
  final void Function()? onTap;

  const UserCard({required this.title, required this.count, this.onTap});

  @override
  Widget build(BuildContext context) => Card();
}

class NotAWidget {
  final String secret;
}
`
	e, ok := extreg.Get("custom_dart_flutter")
	if !ok {
		t.Fatal("custom_dart_flutter not registered")
	}
	ents, err := e.Extract(context.Background(), fi("user_card.dart", "dart", src))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	props := map[string]*types.EntityRecord{}
	var widget *types.EntityRecord
	for i := range ents {
		switch {
		case ents[i].Kind == "SCOPE.Pattern" && ents[i].Subtype == "prop_extraction":
			props[ents[i].Properties["prop_name"]] = &ents[i]
		case ents[i].Kind == "SCOPE.UIComponent" && ents[i].Name == "UserCard":
			widget = &ents[i]
		}
	}

	for _, want := range []string{"title", "count", "onTap"} {
		p, ok := props[want]
		if !ok {
			t.Errorf("expected prop %q", want)
			continue
		}
		if p.Properties["widget"] != "UserCard" {
			t.Errorf("prop %q widget = %q", want, p.Properties["widget"])
		}
	}
	if props["title"] != nil && props["title"].Properties["prop_type"] != "String" {
		t.Errorf("title prop_type = %q, want String", props["title"].Properties["prop_type"])
	}
	if _, ok := props["secret"]; ok {
		t.Error("non-widget class field 'secret' should NOT be a prop")
	}

	if widget == nil {
		t.Fatal("expected UserCard UIComponent")
	}
	hasPropsEdge := 0
	for _, r := range widget.Relationships {
		if r.Kind == string(types.RelationshipKindHasProps) {
			hasPropsEdge++
		}
	}
	if hasPropsEdge != 3 {
		t.Errorf("expected 3 HAS_PROPS edges from UserCard, got %d", hasPropsEdge)
	}
}
