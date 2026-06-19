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

import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  Divider,
  Form,
  Row,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { Search } from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';

const { Title, Text } = Typography;

const initialForm = {
  source_type: 'video_task',
  record_id: '',
  asset_id: '',
  request_id: '',
  id: '',
  type: 'task_id',
  credential_channel_id: '',
};

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

function ResultBlock({ title, value }) {
  return (
    <div style={{ marginBottom: 18 }}>
      <Text strong>{title}</Text>
      <pre style={{ ...codeBlockStyle, marginTop: 8 }}>{formatRaw(value)}</pre>
    </div>
  );
}

const ModerationDiagnose = () => {
  const { t } = useTranslation();
  const [form, setForm] = useState(initialForm);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState(null);
  const [lastSuccess, setLastSuccess] = useState(null);
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
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  useEffect(() => {
    let active = true;
    const loadCredentialChannels = async () => {
      setCredentialChannelsLoading(true);
      try {
        const res = await API.get(
          '/api/admin/moderation/credential-channels',
          {
            skipErrorHandler: true,
          },
        );
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

  const buildPayload = () => {
    if (form.source_type === 'video_task') {
      const recordID = form.record_id.trim();
      if (!recordID) {
        showError('Task record ID / LSF task_id / BP task_id is required');
        return null;
      }
      return {
        source_type: form.source_type,
        record_id: recordID,
      };
    }

    if (form.source_type === 'library_asset') {
      const assetID = form.asset_id.trim();
      if (!assetID) {
        showError('asset_id is required');
        return null;
      }
      return {
        source_type: form.source_type,
        asset_id: assetID,
      };
    }

    const id = form.id.trim();
    if (!id) {
      showError('Id is required');
      return null;
    }
    const credentialChannelID = Number(form.credential_channel_id);
    if (!Number.isInteger(credentialChannelID) || credentialChannelID <= 0) {
      showError('Asset Admin credential channel is required');
      return null;
    }
    return {
      source_type: form.source_type,
      id,
      type: form.type,
      credential_channel_id: credentialChannelID,
    };
  };

  const runDiagnose = async () => {
    const payload = buildPayload();
    if (!payload) {
      return;
    }
    setLoading(true);
    try {
      const res = await API.post('/api/admin/moderation/diagnose', payload, {
        skipErrorHandler: true,
      });
      setResult(res.data?.data || null);
      setLastSuccess(Boolean(res.data?.success));
      if (res.data?.success) {
        showSuccess(t('操作成功'));
      } else {
        showError(res.data?.message || t('操作失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
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
              mode when automatic lookup is unavailable.
            </Text>
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
            field='credential_channel_id'
            optionList={credentialChannels}
            value={form.credential_channel_id}
            loading={credentialChannelsLoading}
            onChange={(value) =>
              updateField('credential_channel_id', value)
            }
          />
        </Col>
      </>
    );
  };

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
                  onClick={runDiagnose}
                >
                  {t('查询')}
                </Button>
                <Button
                  type='tertiary'
                  onClick={() => {
                    setForm(initialForm);
                    setResult(null);
                    setLastSuccess(null);
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
                {lastSuccess !== null && (
                  <Tag color={lastSuccess ? 'green' : 'red'} size='small'>
                    {lastSuccess ? 'success' : 'failure'}
                  </Tag>
                )}
              </Space>
              <ResultBlock
                title='resolved query'
                value={result?.resolved_query}
              />
              <ResultBlock title='resolved details' value={result?.resolved} />
              <ResultBlock
                title='attempted queries'
                value={result?.attempted_queries}
              />
              <ResultBlock
                title='raw request body'
                value={result?.raw_request_body}
              />
              <ResultBlock title='raw response' value={result?.raw_response} />
              <ResultBlock title='raw error' value={result?.raw_error} />
            </Card>
          </Col>
        </Row>
      </div>
    </div>
  );
};

export default ModerationDiagnose;
