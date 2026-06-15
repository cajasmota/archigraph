# services/auth BUILD — auth service targets.
# Used by grafel M6 (issue #2183) Bazel extractor tests.

py_library(
    name = "auth_lib",
    srcs = ["auth.py", "models.py"],
    deps = [
        "//common/utils:string_utils",
        "//services/billing:billing_lib",
        "@pip//:pyjwt",
    ],
    visibility = ["//visibility:public"],
)

py_binary(
    name = "auth_server",
    srcs = ["server.py"],
    main = "server.py",
    deps = [
        ":auth_lib",
        "//common/utils:string_utils",
        "@pip//:flask",
    ],
)

py_test(
    name = "auth_test",
    srcs = ["auth_test.py"],
    deps = [
        ":auth_lib",
        "@pip//:pytest",
    ],
)
