"""Beanie ODM field-level fixture — schema extraction test."""
from typing import Optional, List
from beanie import Document, Link
from pydantic import Field


class Category(Document):
    name: str
    description: Optional[str] = None

    class Settings:
        name = "categories"


class Product(Document):
    title: str
    price: float
    stock: int = 0
    tags: List[str] = []
    category: Optional[str] = None

    class Settings:
        name = "products"
