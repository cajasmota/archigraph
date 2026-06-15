# common/utils BUILD — shared utility targets.
# Used by grafel M6 (issue #2183) Bazel extractor tests.

go_library(
    name = "string_utils",
    srcs = ["strings.go"],
    importpath = "github.com/example/myapp/common/utils",
    visibility = ["//visibility:public"],
    # No internal deps — leaf node.
)

go_test(
    name = "string_utils_test",
    srcs = ["strings_test.go"],
    embed = [":string_utils"],
)
