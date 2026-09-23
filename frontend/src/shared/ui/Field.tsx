import type { ReactNode } from "react";

interface FieldProps {
  id: string;
  label: string;
  required?: boolean;
  hint?: string;
  error?: string;
  children: ReactNode;
}

export function Field({ id, label, required, hint, error, children }: FieldProps) {
  const descriptionId = error ? `${id}-error` : hint ? `${id}-hint` : undefined;
  return (
    <div className={`field${error ? " field-error" : ""}`}>
      <label htmlFor={id}>
        {label}{required && <span className="required" aria-hidden="true"> *</span>}
      </label>
      {children}
      {error ? <span className="field-message" id={descriptionId}>{error}</span> : hint ? <span className="field-hint" id={descriptionId}>{hint}</span> : null}
    </div>
  );
}
