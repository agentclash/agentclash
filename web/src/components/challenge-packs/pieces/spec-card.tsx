"use client";

import { usePackDraft } from "../use-pack-draft";
import { SpecCardView } from "./spec-card-view";
import { ValidationPanel } from "./validation-panel";

/**
 * Builder preview pane: binds the live draft compile state to the shared
 * SpecCardView and appends a validation footer. All visual rendering lives in
 * SpecCardView so the catalog/library reuses the identical card.
 */
export function SpecCard() {
  const { state } = usePackDraft();
  const compile = state.compile;

  return (
    <SpecCardView
      card={compile?.spec_card}
      yaml={compile?.yaml}
      compiling={state.compiling}
      footer={<ValidationPanel valid={compile?.valid ?? false} errors={compile?.errors ?? []} />}
    />
  );
}
