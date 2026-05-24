// issue2081_plain_serializer_test.go — Category C: plain serializers.Serializer
// fields without Meta.model emit USES edges (#2081).
//
// Tests in this file cover:
//   - plain Serializer scalar fields emit USES → parent class (rule 5)
//   - custom capitalised field types (non-DRF-scalar) emit USES_SCHEMA → type
//   - ModelSerializer fields (with Meta.model) still emit REFERENCES (regression)
//   - nested *Serializer fields (rule 2) are unaffected

package python_test

import (
	"testing"
)

// TestIssue2081_PlainSerializerScalar_EmitsUsesEdge verifies that a plain
// serializers.Serializer class (no Meta.model) emits a USES → parent-class
// edge for each scalar field, making the field non-orphan.
func TestIssue2081_PlainSerializerScalar_EmitsUsesEdge(t *testing.T) {
	src := `from rest_framework import serializers

class InspectionQueryParamsSerializer(serializers.Serializer):
    status = serializers.CharField(required=False)
    start_date = serializers.DateField(required=False)
    limit = serializers.IntegerField(required=False, default=50)
`
	entities := runPy(t, "client_fixture_b/serializers/inspection_serializer.py", src)

	for _, fieldName := range []string{
		"InspectionQueryParamsSerializer.status",
		"InspectionQueryParamsSerializer.start_date",
		"InspectionQueryParamsSerializer.limit",
	} {
		field := findEnt(t, entities, "SCOPE.Schema", "field", fieldName)
		if !hasRelKind(field, "USES", ":InspectionQueryParamsSerializer") {
			t.Errorf("expected %q to have USES → InspectionQueryParamsSerializer; rels=%+v",
				fieldName, field.Relationships)
		}
	}
}

// TestIssue2081_PlainSerializerCustomFieldType_EmitsUsesSchemaEdge verifies
// that a custom capitalised field type (not in the known DRF scalar list) emits
// USES_SCHEMA → the field type, not USES → parent.
func TestIssue2081_PlainSerializerCustomFieldType_EmitsUsesSchemaEdge(t *testing.T) {
	src := `from rest_framework import serializers
from myapp.fields import MoneyField

class PaymentSerializer(serializers.Serializer):
    amount = MoneyField()
    currency = serializers.CharField()
`
	entities := runPy(t, "client_fixture_b/serializers/payment_serializer.py", src)

	// Custom MoneyField → USES_SCHEMA
	amountField := findEnt(t, entities, "SCOPE.Schema", "field", "PaymentSerializer.amount")
	if !hasRelKind(amountField, "USES_SCHEMA", ":MoneyField") {
		t.Errorf("expected PaymentSerializer.amount to have USES_SCHEMA → MoneyField; rels=%+v",
			amountField.Relationships)
	}

	// Standard CharField → USES → parent
	currencyField := findEnt(t, entities, "SCOPE.Schema", "field", "PaymentSerializer.currency")
	if !hasRelKind(currencyField, "USES", ":PaymentSerializer") {
		t.Errorf("expected PaymentSerializer.currency to have USES → PaymentSerializer; rels=%+v",
			currencyField.Relationships)
	}
}

// TestIssue2081_ModelSerializerFields_StillEmitsReferences verifies regression:
// ModelSerializer fields (rule 4 — with Meta.model) still emit REFERENCES, not USES.
func TestIssue2081_ModelSerializerFields_StillEmitsReferences(t *testing.T) {
	src := `from rest_framework import serializers

class ContractSerializer(serializers.ModelSerializer):
    title = serializers.CharField()
    amount = serializers.DecimalField(max_digits=10, decimal_places=2)

    class Meta:
        model = Contract
        fields = ['title', 'amount']
`
	entities := runPy(t, "client_fixture_b/serializers/contract_serializer.py", src)

	for _, fieldName := range []string{
		"ContractSerializer.title",
		"ContractSerializer.amount",
	} {
		field := findEnt(t, entities, "SCOPE.Schema", "field", fieldName)
		// Must have REFERENCES (rule 4), NOT just USES.
		if !hasRelKind(field, "REFERENCES", ":Contract") {
			t.Errorf("expected %q to have REFERENCES → Contract (rule 4); rels=%+v",
				fieldName, field.Relationships)
		}
		// Must NOT have USES → parent in place of REFERENCES.
		if hasRelKind(field, "USES", ":ContractSerializer") {
			t.Errorf("%q should not have USES → ContractSerializer when REFERENCES is emitted; rels=%+v",
				fieldName, field.Relationships)
		}
	}
}

// TestIssue2081_PlainSerializerNestedRef_UnaffectedByRule5 verifies that a
// nested *Serializer reference (rule 2) is unaffected — it emits REFERENCES,
// not USES.
func TestIssue2081_PlainSerializerNestedRef_UnaffectedByRule5(t *testing.T) {
	src := `from rest_framework import serializers

class AddressSerializer(serializers.Serializer):
    city = serializers.CharField()

class PersonSerializer(serializers.Serializer):
    address = AddressSerializer()
`
	entities := runPy(t, "client_fixture_b/serializers/person_serializer.py", src)

	// address field: rule 2 fires (nested Serializer identifier), so REFERENCES
	addressField := findEnt(t, entities, "SCOPE.Schema", "field", "PersonSerializer.address")
	if !hasRelKind(addressField, "REFERENCES", ":AddressSerializer") {
		t.Errorf("expected PersonSerializer.address to have REFERENCES → AddressSerializer; rels=%+v",
			addressField.Relationships)
	}

	// city field: plain CharField in a plain Serializer → USES → AddressSerializer
	cityField := findEnt(t, entities, "SCOPE.Schema", "field", "AddressSerializer.city")
	if !hasRelKind(cityField, "USES", ":AddressSerializer") {
		t.Errorf("expected AddressSerializer.city to have USES → AddressSerializer; rels=%+v",
			cityField.Relationships)
	}
}

// TestIssue2081_PlainSerializerListField_EmitsUsesEdge verifies that ListField
// (a common plain-serializer field type) gets USES → parent.
func TestIssue2081_PlainSerializerListField_EmitsUsesEdge(t *testing.T) {
	src := `from rest_framework import serializers

class DeviceFiltersSerializer(serializers.Serializer):
    tags = serializers.ListField(child=serializers.CharField())
    ids = serializers.ListField()
`
	entities := runPy(t, "client_fixture_b/serializers/device_serializer.py", src)

	for _, fieldName := range []string{
		"DeviceFiltersSerializer.tags",
		"DeviceFiltersSerializer.ids",
	} {
		field := findEnt(t, entities, "SCOPE.Schema", "field", fieldName)
		if !hasRelKind(field, "USES", ":DeviceFiltersSerializer") {
			t.Errorf("expected %q to have USES → DeviceFiltersSerializer; rels=%+v",
				fieldName, field.Relationships)
		}
	}
}
