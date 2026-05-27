<!-- DO NOT EDIT — generated from docs/coverage.json by 'go run ./tools/coverage gen' -->
# Coverage — language: `java`

Auto-generated. Back to [summary](../summary.md).

- Records: **13**
- Full: **18** · Partial: **5** · Missing: **4** · N/A: **0**

## Records

| ID | Category | Label | Capabilities |
|----|----------|-------|--------------|
| [config.properties](../detail/config.properties.md) | [configuration](../by-category/configuration.md) | .properties (application.properties) | file_parsing=full |
| [lang.java](../detail/lang.java.md) | [language](../by-category/language.md) | Java | call_line_precision=full, core_extraction=full, discriminates_on=missing |
| [lang.java.framework.dropwizard](../detail/lang.java.framework.dropwizard.md) | [http_framework](../by-category/http_framework.md) | Dropwizard (JAX-RS subset) | endpoint_synthesis=partial, handler_attribution=partial |
| [lang.java.framework.jaxrs](../detail/lang.java.framework.jaxrs.md) | [http_framework](../by-category/http_framework.md) | JAX-RS / Jakarta EE | auth_coverage=full, endpoint_synthesis=full, handler_attribution=full |
| [lang.java.framework.micronaut](../detail/lang.java.framework.micronaut.md) | [http_framework](../by-category/http_framework.md) | Micronaut | endpoint_synthesis=missing |
| [lang.java.framework.quarkus](../detail/lang.java.framework.quarkus.md) | [http_framework](../by-category/http_framework.md) | Quarkus (JAX-RS-backed) | auth_coverage=full, endpoint_synthesis=full, handler_attribution=full |
| [lang.java.framework.spring-boot](../detail/lang.java.framework.spring-boot.md) | [http_framework](../by-category/http_framework.md) | Spring Boot / Spring MVC | auth_coverage=full, endpoint_synthesis=full, handler_attribution=full |
| [lang.java.framework.spring-webflux](../detail/lang.java.framework.spring-webflux.md) | [http_framework](../by-category/http_framework.md) | Spring WebFlux | auth_coverage=partial, endpoint_synthesis=full, handler_attribution=full |
| [lang.java.orm.hibernate](../detail/lang.java.orm.hibernate.md) | [orm](../by-category/orm.md) | Hibernate / JPA | model_extraction=full, query_attribution=partial |
| [lang.java.orm.spring-data-jpa](../detail/lang.java.orm.spring-data-jpa.md) | [orm](../by-category/orm.md) | Spring Data JPA | model_extraction=full, query_attribution=partial |
| [pkg.gradle](../detail/pkg.gradle.md) | [package_manager](../by-category/package_manager.md) | build.gradle / build.gradle.kts | lockfile_parsing=missing, manifest_parsing=missing |
| [pkg.pom](../detail/pkg.pom.md) | [package_manager](../by-category/package_manager.md) | pom.xml | manifest_parsing=full |
| [security.auth-java](../detail/security.auth-java.md) | [security](../by-category/security.md) | Auth policy resolver (Java/Kotlin — Phase 1 of #1942) | auth_policy=full |
