export const initialModerationDiagnoseForm = {
  source_type: 'video_task',
  record_id: '',
  asset_id: '',
  request_id: '',
  id: '',
  type: 'task_id',
  asset_admin_channel_id: '',
};

export const initialModerationDiagnoseState = {
  form: initialModerationDiagnoseForm,
  loading: false,
  result: null,
};

export function moderationDiagnoseReducer(state, action) {
  switch (action.type) {
    case 'update_field':
      return {
        ...state,
        form: { ...state.form, [action.field]: action.value },
        result: null,
      };
    case 'query_start':
      return { ...state, loading: true, result: null };
    case 'validation_failed':
    case 'request_failed':
      return { ...state, loading: false, result: null };
    case 'query_complete':
      return { ...state, loading: false, result: action.result || null };
    case 'continue_manual':
      return {
        ...state,
        loading: false,
        result: null,
        form: {
          ...state.form,
          source_type: 'manual',
          id: state.form.asset_id.trim(),
          type: 'asset_id',
          asset_admin_channel_id: '',
        },
      };
    case 'reset':
      return initialModerationDiagnoseState;
    default:
      return state;
  }
}

export function buildModerationDiagnosePayload(form) {
  if (form.source_type === 'video_task') {
    const recordID = form.record_id.trim();
    if (!recordID) {
      return {
        error: 'Task record ID / LSF task_id / BP task_id is required',
      };
    }
    return {
      payload: {
        source_type: form.source_type,
        record_id: recordID,
      },
    };
  }

  if (form.source_type === 'library_asset') {
    return {
      error:
        'Asset ownership mapping is not currently persisted. Use manual mode to query an upstream asset ID.',
    };
  }

  const id = form.id.trim();
  if (!id) {
    return { error: 'Id is required' };
  }

  const credentialChannelID = Number(form.asset_admin_channel_id);
  const hasCredentialChannel =
    Number.isInteger(credentialChannelID) && credentialChannelID > 0;
  if (form.type !== 'task_id' && !hasCredentialChannel) {
    return { error: 'Asset Admin credential channel is required' };
  }

  return {
    payload: {
      source_type: form.source_type,
      id,
      type: form.type,
      ...(hasCredentialChannel
        ? { asset_admin_channel_id: credentialChannelID }
        : {}),
    },
  };
}

export function getModerationResultPresentation(resultStatus) {
  switch (resultStatus) {
    case 'found':
      return { label: 'Result found', color: 'green' };
    case 'not_found':
      return { label: 'No moderation result', color: 'amber' };
    case 'request_failed':
      return { label: 'Request failed', color: 'red' };
    case 'validation_error':
      return { label: 'Validation error', color: 'orange' };
    case 'rate_limited':
      return { label: 'Rate limited', color: 'violet' };
    default:
      return { label: 'Unknown', color: 'grey' };
  }
}
