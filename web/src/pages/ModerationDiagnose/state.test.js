import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import {
  buildModerationDiagnosePayload,
  getModerationResultPresentation,
  initialModerationDiagnoseState,
  moderationDiagnoseReducer,
} from './state';

describe('Moderation Diagnose form state', () => {
  test('manual task_id does not require a credential channel', () => {
    const { payload, error } = buildModerationDiagnosePayload({
      ...initialModerationDiagnoseState.form,
      source_type: 'manual',
      id: 'cgt-external',
      type: 'task_id',
    });

    expect(error).toBeUndefined();
    expect(payload.asset_admin_channel_id).toBeUndefined();
  });

  test('manual asset_id and request_id require a credential channel', () => {
    for (const type of ['asset_id', 'request_id']) {
      const { error } = buildModerationDiagnosePayload({
        ...initialModerationDiagnoseState.form,
        source_type: 'manual',
        id: 'upstream-id',
        type,
      });
      expect(error).toBe('Asset Admin credential channel is required');
    }
  });

  test('continue in manual mode preserves the asset ID and clears results', () => {
    const state = moderationDiagnoseReducer(
      {
        ...initialModerationDiagnoseState,
        result: { result_status: 'found' },
        form: {
          ...initialModerationDiagnoseState.form,
          source_type: 'library_asset',
          asset_id: 'asset-123',
        },
      },
      { type: 'continue_manual' },
    );

    expect(state.form.source_type).toBe('manual');
    expect(state.form.id).toBe('asset-123');
    expect(state.form.type).toBe('asset_id');
    expect(state.result).toBeNull();
  });

  test('field changes and lifecycle failures clear stale results', () => {
    const populated = {
      ...initialModerationDiagnoseState,
      result: { result_status: 'found' },
    };

    expect(
      moderationDiagnoseReducer(populated, {
        type: 'update_field',
        field: 'source_type',
        value: 'manual',
      }).result,
    ).toBeNull();
    expect(
      moderationDiagnoseReducer(populated, {
        type: 'update_field',
        field: 'id',
        value: 'changed',
      }).result,
    ).toBeNull();
    expect(
      moderationDiagnoseReducer(populated, {
        type: 'update_field',
        field: 'type',
        value: 'asset_id',
      }).result,
    ).toBeNull();
    expect(
      moderationDiagnoseReducer(populated, {
        type: 'update_field',
        field: 'asset_admin_channel_id',
        value: 9,
      }).result,
    ).toBeNull();
    expect(
      moderationDiagnoseReducer(populated, { type: 'query_start' }).result,
    ).toBeNull();
    expect(
      moderationDiagnoseReducer(populated, { type: 'validation_failed' })
        .result,
    ).toBeNull();
    expect(
      moderationDiagnoseReducer(populated, { type: 'request_failed' }).result,
    ).toBeNull();
  });

  test('NotFound uses the no-result presentation', () => {
    expect(getModerationResultPresentation('not_found')).toEqual({
      label: 'No moderation result',
      color: 'amber',
    });
  });

  test('advanced diagnostics are collapsed and explanatory text is present', () => {
    const pageSource = readFileSync(new URL('./index.jsx', import.meta.url), {
      encoding: 'utf8',
    });

    expect(pageSource).toContain("header='Advanced diagnostics'");
    expect(pageSource).not.toContain('activeKey=');
    expect(pageSource).toContain(
      'All upstream query attempts, including fallback attempts.',
    );
    expect(pageSource).toContain(
      'The final redacted response returned by BytePlus.',
    );
  });

  test('page does not render sensitive channel configuration', () => {
    const pageSource = readFileSync(new URL('./index.jsx', import.meta.url), {
      encoding: 'utf8',
    });

    expect(pageSource).not.toContain('ProjectName');
    expect(pageSource).not.toContain('AK/SK');
    expect(pageSource).not.toContain('channel settings');
    expect(pageSource).not.toContain("label='Group'");
  });
});
