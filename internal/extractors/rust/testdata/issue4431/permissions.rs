// permissions.rs — Rust mirror of the acme source-of-truth permission map,
// exercising the const/static value-set shapes #4431 must index. Representative,
// not production-copied: the key→value pairs intentionally match the Django
// PERMISSION_PAGES / v3 PermissionPage oracle so a downstream parity-audit can
// diff this Rust map against them.

use phf::phf_map;
use std::collections::HashMap;

/// Plain scalar module constants — aggregated into one ConstantsGroupName set.
const MAX_PERMISSIONS: u32 = 256;
const DEFAULT_SCOPE: &str = "core";
static SERVICE_NAME: &str = "acme";

/// A const slice map: the canonical &[(&str, &str)] key→value table.
const PERMISSION_PAGES: &[(&str, &str)] = &[
    ("core-admin", "Core Admin"),
    ("contract-proposal", "Contract Proposals"),
    ("users", "Users"),
    ("sync", "Sync"),
];

/// A compile-time phf map (static).
static PAGE_LABELS: phf::Map<&'static str, &'static str> = phf_map! {
    "core-admin" => "Core Admin",
    "billing" => "Billing",
    "users" => "Users",
};

/// A data-enum with an explicit discriminant and a data-carrying variant.
#[derive(Debug, Clone)]
pub enum AccessLevel {
    None = 0,
    Read = 1,
    Write = 2,
    Custom(u32),
}

lazy_static::lazy_static! {
    /// A lazily-built HashMap via insert() calls.
    static ref ROUTE_TABLE: HashMap<&'static str, &'static str> = {
        let mut m = HashMap::new();
        m.insert("home", "/");
        m.insert("admin", "/admin");
        m.insert("billing", "/billing");
        m
    };
}
