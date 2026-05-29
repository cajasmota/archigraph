"""MongoEngine ODM field-level fixture — schema extraction test."""
import mongoengine
from mongoengine import Document, EmbeddedDocument, StringField, IntField, FloatField, BooleanField, ListField, EmbeddedDocumentField


mongoengine.connect("mydb")


class Address(EmbeddedDocument):
    street = StringField(required=True)
    city = StringField(max_length=100)
    zipcode = StringField(max_length=20)


class Customer(Document):
    name = StringField(required=True, max_length=200)
    email = StringField(required=True)
    age = IntField(min_value=0)
    balance = FloatField(default=0.0)
    active = BooleanField(default=True)
    address = EmbeddedDocumentField(Address)
    tags = ListField(StringField())

    meta = {"collection": "customers"}
