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
import React, { useEffect, useState, useRef } from 'react';
import { Banner, Button, Form, Row, Col, Spin } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { BookOpen } from 'lucide-react';

export default function SettingsPaymentGatewayInfini(props) {
  const { t } = useTranslation();
  const sectionTitle = props.hideSectionTitle ? undefined : t('Infini 设置');
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    InfiniKeyID: '',
    InfiniSecretKey: '',
    InfiniBaseURL: '',
    InfiniWebhookSecret: '',
    InfiniMinTopUp: 1,
  });
  const formApiRef = useRef(null);

  useEffect(() => {
    if (props.options && formApiRef.current) {
      const currentInputs = {
        InfiniKeyID: props.options.InfiniKeyID || '',
        InfiniSecretKey: props.options.InfiniSecretKey || '',
        InfiniBaseURL: props.options.InfiniBaseURL || '',
        InfiniWebhookSecret: props.options.InfiniWebhookSecret || '',
        InfiniMinTopUp:
          props.options.InfiniMinTopUp !== undefined
            ? parseFloat(props.options.InfiniMinTopUp)
            : 1,
      };

      setInputs(currentInputs);
      formApiRef.current.setValues(currentInputs);
    }
  }, [props.options]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitInfiniSetting = async () => {
    setLoading(true);
    try {
      const options = [];

      if (inputs.InfiniKeyID && inputs.InfiniKeyID !== '') {
        options.push({ key: 'InfiniKeyID', value: inputs.InfiniKeyID });
      }

      if (inputs.InfiniSecretKey && inputs.InfiniSecretKey !== '') {
        options.push({ key: 'InfiniSecretKey', value: inputs.InfiniSecretKey });
      }

      if (inputs.InfiniBaseURL && inputs.InfiniBaseURL !== '') {
        options.push({ key: 'InfiniBaseURL', value: inputs.InfiniBaseURL });
      }

      if (inputs.InfiniWebhookSecret && inputs.InfiniWebhookSecret !== '') {
        options.push({
          key: 'InfiniWebhookSecret',
          value: inputs.InfiniWebhookSecret,
        });
      }

      options.push({
        key: 'InfiniMinTopUp',
        value: inputs.InfiniMinTopUp.toString(),
      });

      const requestQueue = options.map((opt) =>
        API.put('/api/option/', {
          key: opt.key,
          value: opt.value,
        }),
      );

      const results = await Promise.all(requestQueue);

      const errorResults = results.filter((res) => !res.data.success);
      if (errorResults.length > 0) {
        errorResults.forEach((res) => {
          showError(res.data.message);
        });
      } else {
        showSuccess(t('更新成功'));
        props.refresh && props.refresh();
      }
    } catch (error) {
      showError(t('更新失败'));
    }
    setLoading(false);
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleFormChange}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={sectionTitle}>
          <Banner
            type='info'
            icon={<BookOpen size={16} />}
            description={
              <>
                {t('Infini 是一个加密货币支付网关，支持 USDT 等稳定币收款。')}
                <a
                  href='https://developer.infini.money'
                  target='_blank'
                  rel='noreferrer'
                >
                  Infini Developer Docs
                </a>
                <br />
                {t(
                  '配置 Webhook 回调地址为：{server_address}/api/infini/webhook',
                )}
              </>
            }
            style={{ marginBottom: 16 }}
          />
          <Row gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='InfiniKeyID'
                label='Key ID'
                placeholder={t('Infini API Key ID')}
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='InfiniSecretKey'
                label='Secret Key'
                placeholder={t('敏感信息不会发送到前端显示')}
                type='password'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='InfiniBaseURL'
                label={t('API Base URL')}
                placeholder='https://openapi-sandbox.infini.money'
              />
            </Col>
          </Row>
          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.Input
                field='InfiniWebhookSecret'
                label={t('Webhook 签名密钥')}
                placeholder={t('用于验证回调请求的密钥')}
                type='password'
              />
            </Col>
            <Col xs={24} sm={24} md={8} lg={8} xl={8}>
              <Form.InputNumber
                field='InfiniMinTopUp'
                label={t('最低充值金额')}
                placeholder={t('例如：1')}
                min={1}
              />
            </Col>
          </Row>
          <Button onClick={submitInfiniSetting} style={{ marginTop: 16 }}>
            {t('更新 Infini 设置')}
          </Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
