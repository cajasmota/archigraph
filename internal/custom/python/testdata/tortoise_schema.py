"""Tortoise ORM column-level fixture — schema extraction test."""
from tortoise import fields
from tortoise.models import Model


class Tournament(Model):
    id = fields.IntField(pk=True)
    name = fields.CharField(max_length=255)
    created_at = fields.DatetimeField(auto_now_add=True)
    active = fields.BooleanField(default=True)

    class Meta:
        table = "tournament"


class Event(Model):
    id = fields.IntField(pk=True)
    name = fields.CharField(max_length=255)
    prize = fields.DecimalField(max_digits=10, decimal_places=2)
    description = fields.TextField()

    class Meta:
        table = "event"
