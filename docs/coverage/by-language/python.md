<!-- DO NOT EDIT — generated from docs/coverage.json by 'go run ./tools/coverage gen' -->
# Coverage — language: `python`

Auto-generated. Back to [summary](../summary.md).

- Records: **25**
- Full: **29** · Partial: **5** · Missing: **18** · N/A: **0**

## Records

| ID | Category | Label | Capabilities |
|----|----------|-------|--------------|
| [lang.python](../detail/lang.python.md) | [language](../by-category/language.md) | Python | call_line_precision=full, core_extraction=full, discriminates_on=full, navigates_to=missing |
| [lang.python.framework.aiohttp](../detail/lang.python.framework.aiohttp.md) | [http_framework](../by-category/http_framework.md) | aiohttp | endpoint_synthesis=missing |
| [lang.python.framework.bottle](../detail/lang.python.framework.bottle.md) | [http_framework](../by-category/http_framework.md) | Bottle | endpoint_synthesis=missing |
| [lang.python.framework.django](../detail/lang.python.framework.django.md) | [http_framework](../by-category/http_framework.md) | Django (URLconf) | auth_coverage=partial, endpoint_synthesis=full, handler_attribution=full |
| [lang.python.framework.django-drf](../detail/lang.python.framework.django-drf.md) | [http_framework](../by-category/http_framework.md) | Django REST Framework | auth_coverage=partial, endpoint_synthesis=full, handler_attribution=full |
| [lang.python.framework.fastapi](../detail/lang.python.framework.fastapi.md) | [http_framework](../by-category/http_framework.md) | FastAPI | auth_coverage=missing, endpoint_synthesis=full |
| [lang.python.framework.flask](../detail/lang.python.framework.flask.md) | [http_framework](../by-category/http_framework.md) | Flask | auth_coverage=missing, endpoint_synthesis=full |
| [lang.python.framework.litestar](../detail/lang.python.framework.litestar.md) | [http_framework](../by-category/http_framework.md) | Litestar | endpoint_synthesis=missing |
| [lang.python.framework.pyramid](../detail/lang.python.framework.pyramid.md) | [http_framework](../by-category/http_framework.md) | Pyramid | endpoint_synthesis=full, handler_attribution=full |
| [lang.python.framework.robyn](../detail/lang.python.framework.robyn.md) | [http_framework](../by-category/http_framework.md) | Robyn | endpoint_synthesis=missing |
| [lang.python.framework.sanic](../detail/lang.python.framework.sanic.md) | [http_framework](../by-category/http_framework.md) | Sanic | endpoint_synthesis=missing |
| [lang.python.framework.starlette](../detail/lang.python.framework.starlette.md) | [http_framework](../by-category/http_framework.md) | Starlette | endpoint_synthesis=full, handler_attribution=full |
| [lang.python.framework.tornado](../detail/lang.python.framework.tornado.md) | [http_framework](../by-category/http_framework.md) | Tornado | endpoint_synthesis=full, handler_attribution=full |
| [lang.python.orm.django](../detail/lang.python.orm.django.md) | [orm](../by-category/orm.md) | Django ORM | migration_parsing=full, model_extraction=full, query_attribution=full |
| [lang.python.orm.mongoengine](../detail/lang.python.orm.mongoengine.md) | [orm](../by-category/orm.md) | MongoEngine | model_extraction=missing, query_attribution=missing |
| [lang.python.orm.peewee](../detail/lang.python.orm.peewee.md) | [orm](../by-category/orm.md) | Peewee | model_extraction=missing, query_attribution=full |
| [lang.python.orm.sqlalchemy](../detail/lang.python.orm.sqlalchemy.md) | [orm](../by-category/orm.md) | SQLAlchemy | migration_parsing=partial, model_extraction=full, query_attribution=full |
| [lang.python.orm.sqlmodel](../detail/lang.python.orm.sqlmodel.md) | [orm](../by-category/orm.md) | SQLModel | model_extraction=missing, query_attribution=partial |
| [lang.python.orm.tortoise](../detail/lang.python.orm.tortoise.md) | [orm](../by-category/orm.md) | Tortoise ORM | model_extraction=missing, query_attribution=full |
| [msg.celery](../detail/msg.celery.md) | [message_broker](../by-category/message_broker.md) | Celery (Python task queue) | consumer_extraction=full, producer_extraction=full, topic_attribution=full |
| [msg.django-signals](../detail/msg.django-signals.md) | [message_broker](../by-category/message_broker.md) | Django signals (intra-repo pub/sub) | consumer_extraction=full, producer_extraction=full, topic_attribution=partial |
| [msg.dramatiq](../detail/msg.dramatiq.md) | [message_broker](../by-category/message_broker.md) | Dramatiq (Python task queue) | consumer_extraction=missing, producer_extraction=missing |
| [pkg.pipfile](../detail/pkg.pipfile.md) | [package_manager](../by-category/package_manager.md) | Pipfile / Pipfile.lock | lockfile_parsing=missing, manifest_parsing=missing |
| [pkg.pyproject](../detail/pkg.pyproject.md) | [package_manager](../by-category/package_manager.md) | pyproject.toml | lockfile_parsing=missing, manifest_parsing=full |
| [pkg.requirements](../detail/pkg.requirements.md) | [package_manager](../by-category/package_manager.md) | requirements.txt | manifest_parsing=full |
