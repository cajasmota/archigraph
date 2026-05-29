"""Fixture: Python logging with stdlib logging, loguru, and structlog."""

# --- stdlib logging ---
import logging

logger = logging.getLogger(__name__)
app_log = logging.getLogger("myapp")

logger.info("Server started on port %d", 8080)
logger.debug("Incoming request: %s %s", method, path)
logger.warning("Rate limit exceeded for user %s", user_id)
logger.error("Database connection failed: %s", exc)
app_log.critical("Unrecoverable error, shutting down")

# --- loguru ---
from loguru import logger as log

log.info("loguru info message")
log.debug("loguru debug")
bound = logger.bind(request_id="abc")

# --- structlog ---
import structlog

structlog.configure(processors=[structlog.processors.JSONRenderer()])
struct_logger = structlog.get_logger()
struct_logger.info("structlog info", user=user_id)
struct_logger.warning("structlog warn")
