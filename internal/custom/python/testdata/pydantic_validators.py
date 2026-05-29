"""Dependency-free proving fixture for the Pydantic extractor (issue #2984).

Exercises Pydantic v2 field/model validators, Field(...) constraints, and
ConfigDict-based coercion config, plus a v1-style class for dialect coverage.
The imports are written so the file parses but needs no installed packages.
"""

from pydantic import BaseModel, Field, field_validator, model_validator, ConfigDict


class SignupRequest(BaseModel):
    model_config = ConfigDict(strict=True, str_strip_whitespace=True)

    username: str = Field(min_length=3, max_length=32, pattern=r"^[a-z0-9_]+$")
    age: int = Field(gt=0, le=150)
    email: str

    @field_validator("email", mode="before")
    @classmethod
    def normalize_email(cls, v):
        return v.strip().lower()

    @model_validator(mode="after")
    def check_consistency(self):
        return self


class LegacyModel(BaseModel):
    score: int = Field(ge=0, le=100)

    class Config:
        allow_population_by_field_name = True
        use_enum_values = True

    @validator("score", pre=True)
    def coerce_score(cls, v):
        return int(v)
