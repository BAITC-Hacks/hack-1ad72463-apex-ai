import { z } from "zod";

const dateOnlySchema = z.string().regex(/^\d{4}-\d{2}-\d{2}$/);
const nonNegativeInteger = z.number().int().nonnegative();

const recommendationSchema = z.object({
  id: z.string().min(1),
  anon_name: z.string().min(1),
  city: z.string().min(1),
  category: z.string().min(1),
  price_from_kzt: z.number().int().positive().safe(),
  max_hours: z.number().positive().finite().nullable(),
  synthetic: z.boolean(),
  price_imputed: z.boolean(),
  city_imputed: z.boolean(),
  explanation: z.string().min(1),
  conditions_to_confirm: z.array(z.object({ fact_id: z.string().min(1), text: z.string().min(1) }).strict()).optional(),
}).strict();

export const searchResponseSchema = z.object({
  status: z.enum(["MATCHES_FOUND", "NO_CATALOG", "NO_MATCH"]),
  message: z.string().min(1),
  results: z.array(recommendationSchema).max(3),
  diagnostics: z.object({
    catalog_count: nonNegativeInteger,
    eligible_count: nonNegativeInteger,
    returned_count: nonNegativeInteger.max(3),
    excluded_counts: z.object({
      EVENT_FORMAT_UNSUPPORTED: nonNegativeInteger,
      BUDGET_TOO_LOW: nonNegativeInteger,
      LANGUAGE_UNSUPPORTED: nonNegativeInteger,
      DURATION_EXCEEDED: nonNegativeInteger,
      BUSY_ON_DATE: nonNegativeInteger,
    }).strict(),
    omitted_optional_checks: z.array(z.enum(["language", "duration_hours"])),
    applied_order: z.array(z.string().min(1)),
  }).strict(),
  metadata: z.object({
    snapshot_version: z.string().min(1),
    loader_version: z.string().min(1),
    policy_version: z.string().min(1),
    facts_version: z.string().min(1),
    calendar_window: z.object({ from: dateOnlySchema, to: dateOnlySchema }).strict(),
  }).strict(),
}).strict().superRefine((response, context) => {
  const { diagnostics, results, status } = response;
  const excluded = Object.values(diagnostics.excluded_counts).reduce((sum, count) => sum + count, 0);

  if (results.length !== diagnostics.returned_count) {
    context.addIssue({ code: "custom", message: "returned_count does not match results length" });
  }
  if (diagnostics.returned_count !== Math.min(diagnostics.eligible_count, 3)) {
    context.addIssue({ code: "custom", message: "returned_count does not match eligible_count" });
  }
  if (excluded + diagnostics.eligible_count !== diagnostics.catalog_count) {
    context.addIssue({ code: "custom", message: "diagnostic counters are inconsistent" });
  }
  if (status === "MATCHES_FOUND" && (diagnostics.eligible_count < 1 || results.length < 1)) {
    context.addIssue({ code: "custom", message: "MATCHES_FOUND requires results" });
  }
  if (status === "NO_CATALOG" && (diagnostics.catalog_count !== 0 || diagnostics.eligible_count !== 0 || results.length !== 0)) {
    context.addIssue({ code: "custom", message: "NO_CATALOG counters are inconsistent" });
  }
  if (status === "NO_MATCH" && (diagnostics.catalog_count < 1 || diagnostics.eligible_count !== 0 || results.length !== 0)) {
    context.addIssue({ code: "custom", message: "NO_MATCH counters are inconsistent" });
  }
});

export const errorResponseSchema = z.object({
  error: z.object({
    code: z.string().min(1),
    details: z.array(z.object({
      field: z.string(),
      code: z.string(),
      message: z.string(),
    }).strict()),
  }).strict(),
}).strict();
