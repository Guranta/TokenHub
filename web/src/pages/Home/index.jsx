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

import React, { useContext, useEffect, useState } from 'react';
import {
  Button,
} from '@douyinfe/semi-ui';
import { API, showError } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { StatusContext } from '../../context/Status';
import { useActualTheme } from '../../context/Theme';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import {
  IconPlay,
} from '@douyinfe/semi-icons';
import { Link } from 'react-router-dom';
import NoticeModal from '../../components/layout/NoticeModal';

const Home = () => {
  const { t, i18n } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const isMobile = useIsMobile();

  const displayHomePageContent = async () => {
    setHomePageContent(localStorage.getItem('home_page_content') || '');
    try {
      const res = await API.get('/api/home_page_content');
      const { success, message, data } = res.data;
      if (success) {
        let content = data;
        if (!data.startsWith('https://')) {
          content = marked.parse(data);
        }
        setHomePageContent(content);
        localStorage.setItem('home_page_content', content);

        if (data.startsWith('https://')) {
          const iframe = document.querySelector('iframe');
          if (iframe) {
            iframe.onload = () => {
              iframe.contentWindow.postMessage({ themeMode: actualTheme }, '*');
              iframe.contentWindow.postMessage({ lang: i18n.language }, '*');
            };
          }
        }
      } else {
        setHomePageContent('');
      }
    } catch {
      setHomePageContent('');
    }
    setHomePageContentLoaded(true);
  };

  useEffect(() => {
    const checkNoticeAndShow = async () => {
      const lastCloseDate = localStorage.getItem('notice_close_date');
      const today = new Date().toDateString();
      if (lastCloseDate !== today) {
        try {
          const res = await API.get('/api/notice');
          const { success, data } = res.data;
          if (success && data && data.trim() !== '') {
            setNoticeVisible(true);
          }
        } catch (error) {
          console.error('获取公告失败:', error);
        }
      }
    };

    checkNoticeAndShow();
  }, []);

  useEffect(() => {
    displayHomePageContent().then();
  }, []);

  return (
    <div className='w-full overflow-x-hidden'>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      {homePageContentLoaded && homePageContent === '' ? (
        <div className='w-full overflow-x-hidden min-h-[80vh] flex items-center justify-center'>
          <div className='flex flex-col items-center justify-center text-center px-4 tokenhub-hero'>
            <div className='tokenhub-hero-icon'>
              <svg
                width='64'
                height='64'
                viewBox='0 0 512 512'
                fill='none'
                xmlns='http://www.w3.org/2000/svg'
                style={{ filter: 'var(--tokenhub-icon-glow, none)' }}
              >
                <rect width='512' height='512' fill='transparent' />
                <g
                  fill='none'
                  stroke='var(--semi-color-text-0)'
                  strokeWidth='1.5'
                  strokeLinecap='square'
                  strokeLinejoin='miter'
                >
                  <rect
                    x='120'
                    y='220'
                    width='72'
                    height='72'
                    fill='var(--semi-color-text-0)'
                    stroke='none'
                  />
                  <line x1='192' y1='256' x2='300' y2='256' />
                  <line x1='300' y1='256' x2='400' y2='168' className='tokenhub-line-1' />
                  <line x1='300' y1='256' x2='400' y2='256' className='tokenhub-line-2' />
                  <line x1='300' y1='256' x2='400' y2='344' className='tokenhub-line-3' />
                  <line x1='400' y1='168' x2='400' y2='344' className='tokenhub-line-bar' />
                </g>
              </svg>
            </div>
            <h1
              className='text-5xl md:text-7xl lg:text-8xl font-bold tracking-tight whitespace-nowrap tokenhub-hero-title'
              style={{ color: 'var(--semi-color-text-0)' }}
            >
              Open Token To Everyone
            </h1>
            <p
              className='text-base md:text-lg mt-4 tokenhub-hero-sub'
              style={{ color: 'var(--semi-color-text-2)' }}
            >
              Help Someone Change The World
            </p>
            <div className='tokenhub-hero-cta'>
              <Link to='/console'>
                <Button
                  theme='solid'
                  type='primary'
                  size='large'
                  className='!rounded-3xl px-8 py-2'
                  icon={<IconPlay />}
                >
                  {t('获取密钥')}
                </Button>
              </Link>
            </div>
          </div>
        </div>
      ) : (
        <div className='overflow-x-hidden w-full'>
          {homePageContent.startsWith('https://') ? (
            <iframe
              src={homePageContent}
              className='w-full h-screen border-none'
            />
          ) : (
            <div
              className='mt-[60px]'
              dangerouslySetInnerHTML={{ __html: homePageContent }}
            />
          )}
        </div>
      )}
    </div>
  );
};

export default Home;
