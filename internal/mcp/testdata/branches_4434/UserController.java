package com.example.api;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@RestController
public class UserController {

    private static final Logger log = LoggerFactory.getLogger(UserController.class);

    private final UserRepository repo;

    public UserController(UserRepository repo) {
        this.repo = repo;
    }

    // create — Spring controller method with an env-gate (System.getenv),
    // an early-return guard returning ResponseEntity.status(HttpStatus.BAD_REQUEST),
    // and a try/catch that logs then returns a 500 ResponseEntity.
    @PostMapping("/users")
    public ResponseEntity<?> create(@RequestBody UserDto dto) {
        if (System.getenv("SIGNUP_ENABLED") == null) {
            return ResponseEntity.status(503).build();
        }

        if (dto.getEmail() == null) {
            return ResponseEntity.status(HttpStatus.BAD_REQUEST).body("email required");
        }

        try {
            if (repo.existsByEmail(dto.getEmail())) {
                return ResponseEntity.status(HttpStatus.CONFLICT).body("email in use");
            }
            User saved = repo.save(dto.toEntity());
            return ResponseEntity.status(HttpStatus.CREATED).body(saved);
        } catch (Exception e) {
            log.error("create failed", e);
            return ResponseEntity.status(500).body("internal error");
        }
    }
}
