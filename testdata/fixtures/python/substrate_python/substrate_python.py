# Python substrate proving fixture (#3068).
#
# Proves that the framework-blind Python substrate sniffers fire on
# representative Python http-framework source code:
#
#   - http_effect               — fetch_user calls requests.get()
#   - db_effect (read)          — list_items uses ORM .filter() read
#   - db_effect (write)         — save_item calls .save()
#   - fs_effect (read)          — read_config uses open() for reading
#   - fs_effect (write)         — write_log uses open() for writing
#   - mutation_effect           — UserService.set_user assigns self.user
#   - confidence_overlay        — effect sniffer emits non-zero confidence
#                                 for each match; effect_propagation pass
#                                 consumes these confidence values
#
import os
import requests


# ── pure helper ───────────────────────────────────────────────────────────────
# Proves pure_function_tagging: no side effects on format_label.
def format_label(name: str) -> str:
    return f"[{name}]"


# ── http_effect ───────────────────────────────────────────────────────────────
# Proves http_effect: fetch_user calls requests.get.
def fetch_user(user_id: int):
    r = requests.get(f"https://api.example.com/users/{user_id}")
    return r.json()


# ── db_effect (read) ──────────────────────────────────────────────────────────
# Proves db_effect read: list_items uses Django ORM .filter().
def list_items(active: bool):
    return Item.objects.filter(active=active)


# ── db_effect (write) ─────────────────────────────────────────────────────────
# Proves db_effect write: save_item calls .save() (ORM write).
def save_item(item):
    item.save()


# ── fs_effect (read) ──────────────────────────────────────────────────────────
# Proves fs_effect read: read_config opens file for reading.
def read_config(path: str) -> str:
    with open(path) as f:
        return f.read()


# ── fs_effect (write) ─────────────────────────────────────────────────────────
# Proves fs_effect write: write_log opens file for writing.
def write_log(path: str, msg: str) -> None:
    with open(path, "w") as f:
        f.write(msg)


# ── mutation_effect ───────────────────────────────────────────────────────────
# Proves mutation_effect: set_user assigns self.user (field mutation).
class UserService:
    def set_user(self, user):
        self.user = user

    def get_user(self):
        return self.user
