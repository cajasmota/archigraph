"""Peewee ORM field-level fixture — schema extraction test."""
import peewee
from peewee import Model, CharField, IntegerField, FloatField, BooleanField, DateTimeField, TextField


database = peewee.SqliteDatabase("app.db")


class BaseModel(Model):
    class Meta:
        database = database


class User(BaseModel):
    username = CharField(max_length=100, unique=True)
    email = CharField(max_length=255)
    age = IntegerField(null=True)
    active = BooleanField(default=True)
    created_at = DateTimeField()


class Post(BaseModel):
    title = CharField(max_length=200)
    body = TextField()
    score = FloatField(default=0.0)
    published = BooleanField(default=False)
