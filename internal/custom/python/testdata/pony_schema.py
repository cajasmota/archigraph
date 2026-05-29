"""Pony ORM attribute-level fixture — schema extraction test."""
from pony.orm import Database, Required, Optional, PrimaryKey, Set

db = Database()


class Author(db.Entity):
    name = Required(str)
    email = Optional(str)
    books = Set("Book")


class Book(db.Entity):
    title = Required(str)
    isbn = PrimaryKey(str)
    year = Optional(int)
    price = Optional(float)
    author = Required(Author)
