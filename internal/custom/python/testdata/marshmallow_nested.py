"""Dependency-free proving fixture for the marshmallow extractor (issue #2985, #3077).

Exercises marshmallow Schema class declarations, field types, Nested() fields,
@validates, @validates_schema, @post_load coercion hooks, and constraint
extraction via validate.Range() / validate.Length() / validate.OneOf().
"""
from marshmallow import Schema, fields, validates, validates_schema, post_load, pre_load, validate, ValidationError


class AddressSchema(Schema):
    street = fields.Str(required=True)
    city = fields.Str(required=True)
    zip_code = fields.Str()


class UserSchema(Schema):
    name = fields.Str(required=True)
    email = fields.Email(required=True)
    age = fields.Int()
    # Nested schema reference — proves nested_model_extraction
    address = fields.Nested(AddressSchema)
    # Many nested
    orders = fields.Nested("OrderSchema", many=True)

    @validates("email")
    def validate_email(self, value):
        if "@" not in value:
            raise ValidationError("Not a valid email.")

    @validates_schema
    def validate_name_age(self, data, **kwargs):
        if data.get("age") and data["age"] < 0:
            raise ValidationError("Age must be positive.")

    @post_load
    def make_user(self, data, **kwargs):
        return User(**data)


class OrderSchema(Schema):
    amount = fields.Float(required=True, validate=lambda x: x > 0)
    currency = fields.Str(load_default="USD")
    user = fields.Nested(UserSchema, load_only=True)

    @pre_load
    def normalize_amount(self, data, **kwargs):
        data["amount"] = float(data.get("amount", 0))
        return data


# Constraint extraction evidence — Range / Length / OneOf (issue #3077)
class ProductSchema(Schema):
    price = fields.Float(validate=validate.Range(min=0, max=99999))
    name = fields.Str(validate=validate.Length(min=1, max=100))
    status = fields.Str(validate=validate.OneOf(["active", "inactive", "pending"]))
