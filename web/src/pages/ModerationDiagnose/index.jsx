/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useMemo, useReducer, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Button,
  Card,
  Col,
  Collapse,
  Divider,
  Empty,
  Form,
  Row,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { Search } from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';
import {
  buildModerationDiagnosePayload,
  getModerationResultPresentation,
  initialModerationDiagnoseState,
  moderationDiagnoseReducer,
} from './state';

const { Title, Text } = Typography;

const codeBlockStyle = {
  background: 'var(--semi-color-bg-2)',
  border: '1px solid var(--semi-color-border)',
  borderRadius: 6,
  fontFamily: 'monospace',
  fontSize: 12,
  lineHeight: 1.55,
  margin: 0,
  maxHeight: 360,
  minHeight: 44,
  overflow: 'auto',
  padding: '10px 12px',
  whiteSpace: 'pre-wrap',
  wordBreak: 'break-word',
};

function formatRaw(value) {
  if (value === undefined || value === null || value === '') {
    return '-';
  }
  if (typeof value !== 'string') {
    return JSON.stringify(value, null, 2);
  }
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch (e) {
    return value;
  }
}

function ResultBlock({ title, description, value }) {
  return (
    <div style={{ marginBottom: 18 }}>
      <Text strong>{title}</Text>
      {description && (
        <div style={{ marginTop: 4 }}>
          <Text type='tertiary'>{description}</Text>
        </div>
      )}
      <pre style={{ ...codeBlockStyle, marginTop: 8 }}>{formatRaw(value)}</pre>
    </div>
  );
}

const ModerationDiagnose = () => {
  const { t } = useTranslation();
  const [state, dispatch] = useReducer(
    moderationDiagnoseReducer,
    initialModerationDiagnoseState,
  );
  const { form, loading, result } = state;
  const [credentialChannels, setCredentialChannels] = useState([]);
  const [credentialChannelsLoading, setCredentialChannelsLoading] =
    useState(false);

  const sourceOptions = useMemo(
    () => [
      { label: 'video_task', value: 'video_task' },
      { label: 'library_asset', value: 'library_asset' },
      { label: 'manual', value: 'manual' },
    ],
    [],
  );

  const typeOptions = useMemo(
    () => [
      { label: 'task_id', value: 'task_id' },
      { label: 'request_id', value: 'request_id' },
      { label: 'asset_id', value: 'asset_id' },
    ],
    [],
  );

  const updateField = (key, value) => {
    dispatch({ type: 'update_field', field: key, value });
  };

  useEffect(() => {
    let active = true;
    const loadCredentialChannels = async () => {
      setCredentialChannelsLoading(true);
      try {
        const res = await API.get('/api/admin/moderation/credential-channels', {
          skipErrorHandler: true,
        });
        if (!active) return;
        const options = (res.data?.data || []).map((item) => ({
          label: item.label,
          value: item.id,
        }));
        setCredentialChannels(options);
      } catch (error) {
        if (active) {
          showError(error);
        }
      } finally {
        if (active) {
          setCredentialChannelsLoading(false);
        }
      }
    };
    loadCredentialChannels();
    return () => {
      active = false;
    };
  }, []);

  const runDiagnose = async () => {
    dispatch({ type: 'query_start' });
    const { payload, error } = buildModerationDiagnosePayload(form);
    if (error) {
      dispatch({ type: 'validation_failed' });
      showError(error);
      return;
    }
    try {
      const res = await API.post('/api/admin/moderation/diagnose', payload, {
        skipErrorHandler: true,
      });
      const nextResult = res.data?.data || null;
      dispatch({ type: 'query_complete', result: nextResult });
      if (nextResult?.result_status === 'found') {
        showSuccess(t('操作成功'));
      } else if (
        nextResult?.result_status === 'request_failed' ||
        nextResult?.result_status === 'validation_error'
      ) {
        showError(res.data?.message || t('操作失败'));
      }
    } catch (error) {
      const rateLimitResult = error?.response?.data?.data;
      if (rateLimitResult?.result_status === 'rate_limited') {
        dispatch({ type: 'query_complete', result: rateLimitResult });
      } else {
        dispatch({ type: 'request_failed' });
      }
      showError(error);
    }
  };

  const continueLibraryAssetInManualMode = () => {
    dispatch({ type: 'continue_manual' });
  };

  const renderSourceFields = () => {
    if (form.source_type === 'video_task') {
      return (
        <>
          <Col xs={24} md={12}>
            <Form.Input
              label='Task record ID / LSF task_id / BP task_id'
              field='record_id'
              value={form.record_id}
              onChange={(value) => updateField('record_id', value)}
            />
          </Col>
        </>
      );
    }

    if (form.source_type === 'library_asset') {
      return (
        <>
          <Col xs={24} md={12}>
            <Form.Input
              label='asset_id'
              field='asset_id'
              value={form.asset_id}
              onChange={(value) => updateField('asset_id', value)}
            />
          </Col>
          <Col xs={24}>
            <Text type='tertiary'>
              Asset ownership mapping is not currently persisted. Use manual
              mode to query an upstream asset ID.
            </Text>
          </Col>
          <Col xs={24}>
            <Button onClick={continueLibraryAssetInManualMode}>
              Continue in manual mode
            </Button>
          </Col>
        </>
      );
    }

    return (
      <>
        <Col xs={24} md={12}>
          <Form.Input
            label='Id'
            field='id'
            value={form.id}
            onChange={(value) => updateField('id', value)}
          />
        </Col>
        <Col xs={24} md={12}>
          <Form.Select
            label='Type'
            field='type'
            optionList={typeOptions}
            value={form.type}
            onChange={(value) => updateField('type', value)}
          />
        </Col>
        <Col xs={24} md={12}>
          <Form.Select
            label='Asset Admin credential channel'
            field='asset_admin_channel_id'
            optionList={credentialChannels}
            value={form.asset_admin_channel_id}
            loading={credentialChannelsLoading}
            onChange={(value) => updateField('asset_admin_channel_id', value)}
          />
        </Col>
        <Col xs={24}>
          <Text type='tertiary'>
            {form.type === 'task_id'
              ? 'Known LSF/BP tasks are resolved automatically. Select a credential channel only for external task IDs.'
              : 'Select the Asset Admin credential channel that owns this upstream ID.'}
          </Text>
        </Col>
      </>
    );
  };

  const presentation = getModerationResultPresentation(result?.result_status);

  return (
    <div className='mt-[60px] px-2'>
      <div style={{ maxWidth: 1180, margin: '0 auto' }}>
        <Title heading={4} style={{ marginBottom: 18 }}>
          Moderation Diagnose
        </Title>
        <Row gutter={[16, 16]}>
          <Col xs={24} lg={9}>
            <Card bodyStyle={{ padding: 18 }}>
              <Form layout='vertical' initValues={form}>
                <Row gutter={[12, 12]}>
                  <Col xs={24}>
                    <Form.Select
                      label='source_type'
                      field='source_type'
                      optionList={sourceOptions}
                      value={form.source_type}
                      onChange={(value) => updateField('source_type', value)}
                    />
                  </Col>
                  {renderSourceFields()}
                </Row>
              </Form>
              <Divider margin='16px' />
              <Space>
                <Button
                  type='primary'
                  icon={<Search size={16} />}
                  loading={loading}
                  disabled={form.source_type === 'library_asset'}
                  onClick={runDiagnose}
                >
                  {t('查询')}
                </Button>
                <Button
                  type='tertiary'
                  onClick={() => {
                    dispatch({ type: 'reset' });
                  }}
                >
                  {t('重置')}
                </Button>
              </Space>
            </Card>
          </Col>
          <Col xs={24} lg={15}>
            <Card bodyStyle={{ padding: 18 }}>
              <Space style={{ marginBottom: 16 }}>
                <Text strong>{t('诊断结果')}</Text>
                {result && (
                  <Tag color={presentation.color} size='small'>
                    {presentation.label}
                  </Tag>
                )}
              </Space>
              {!result ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description='No diagnostic result yet.'
                />
              ) : (
                <>
                  <ResultBlock
                    title='Result status'
                    value={presentation.label}
                  />
                  {result.result_status === 'not_found' && (
                    <Banner
                      type='info'
                      style={{ marginBottom: 18 }}
                      description='The ID may be invalid, not moderation-blocked, outside the 14-day window, or created before whitelist activation.'
                    />
                  )}
                  <ResultBlock
                    title='Resolved query'
                    value={result.resolved_query}
                  />
                  <ResultBlock
                    title='Resolved details'
                    value={result.resolved}
                  />
                  <ResultBlock
                    title='Raw response'
                    description='The final redacted response returned by BytePlus.'
                    value={result.raw_response}
                  />
                  <Collapse>
                    <Collapse.Panel
                      header='Advanced diagnostics'
                      itemKey='advanced-diagnostics'
                    >
                      <ResultBlock
                        title='Attempted queries'
                        description='All upstream query attempts, including fallback attempts.'
                        value={result.attempted_queries}
                      />
                      <ResultBlock
                        title='Raw request body'
                        value={result.raw_request_body}
                      />
                      <ResultBlock title='Raw error' value={result.raw_error} />
                    </Collapse.Panel>
                  </Collapse>
                </>
              )}
            </Card>
          </Col>
        </Row>
      </div>
    </div>
  );
};

export default ModerationDiagnose;
