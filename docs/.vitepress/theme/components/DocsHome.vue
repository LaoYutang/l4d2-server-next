<script setup lang="ts">
import { withBase } from 'vitepress';

const guides = [
  {
    title: '安装部署',
    description: '快速搭建，或接管已有服务器。',
    link: '/guide/quick-start',
    paths: ['M4 4h16v16H4z', 'M4 8h16', 'm8 12 3 3-3 3', 'M13 18h4']
  },
  {
    title: '功能指南',
    description: '熟悉地图、插件与监控。',
    link: '/features/dashboard',
    paths: ['M4 6h7m4 0h5', 'M4 12h3m4 0h9', 'M4 18h11m4 0h1', 'M11 4v4h4V4z', 'M7 10v4h4v-4z', 'M15 16v4h4v-4z']
  },
  {
    title: '日常运维',
    description: '备份迁移与常见问题排查。',
    link: '/operations/migration',
    paths: ['M12 3 4 6v6c0 4 3 7 8 9 5-2 8-5 8-9V6z']
  }
];

// Captures of the existing management panel, with sample data in both themes.
const previews = [
  { title: '服务器状态', page: 'dashboard', link: '/features/dashboard' },
  { title: '地图管理', page: 'maps', link: '/features/maps' },
  { title: '插件管理', page: 'plugins', link: '/features/plugins' },
  { title: '性能监控', page: 'monitor', link: '/features/monitor' }
];
</script>

<template>
  <div class="docs-home">
    <section class="home-hero" aria-labelledby="home-title">
      <p class="home-eyebrow">使用手册</p>
      <h1 id="home-title">L4D2 Server <span>Next</span></h1>
      <p class="home-description">从安装部署到日常运维，一份完整指南。</p>
      <div class="home-actions">
        <a class="home-button primary" :href="withBase('/guide/quick-start')">
          快速开始
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M5 12h14m-6-6 6 6-6 6" />
          </svg>
        </a>
        <a class="home-button secondary" :href="withBase('/features/dashboard')">功能指南</a>
      </div>
    </section>

    <nav class="home-guides" aria-label="文档分类">
      <a v-for="guide in guides" :key="guide.link" class="guide-card" :href="withBase(guide.link)">
        <div class="guide-top">
          <span class="guide-icon">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path v-for="path in guide.paths" :key="path" :d="path" />
            </svg>
          </span>
          <svg class="guide-arrow" viewBox="0 0 24 24" aria-hidden="true">
            <path d="m9 5 7 7-7 7" />
          </svg>
        </div>
        <h2>{{ guide.title }}</h2>
        <p>{{ guide.description }}</p>
      </a>
    </nav>

    <section class="home-previews" aria-labelledby="preview-title">
      <h2 id="preview-title">面板预览</h2>
      <div class="preview-grid">
        <a v-for="preview in previews" :key="preview.page" class="preview-card" :href="withBase(preview.link)">
          <div class="preview-image">
            <img
              v-for="theme in ['light', 'dark']"
              :key="theme"
              :class="`preview-${theme}`"
              :src="withBase(`/previews/${preview.page}-${theme}.jpg`)"
              :alt="`${preview.title}面板（示例数据）`"
              width="1280"
              height="800"
              loading="lazy"
              decoding="async"
            />
          </div>
          <h3>{{ preview.title }}</h3>
        </a>
      </div>
    </section>
  </div>
</template>

<style scoped>
.docs-home {
  --home-card-bg: var(--vp-c-bg);
  --home-icon-bg: #eff6ff;
  --home-icon-color: #2563eb;
  --home-border: var(--vp-c-divider);
  --home-hover-border: #93c5fd;
  --home-title: #172033;
  --home-accent: #2563eb;
  max-width: 1184px;
  margin: 0 auto;
  padding: 0 32px 48px;
}

:global(.dark .docs-home) {
  --home-card-bg: var(--vp-c-bg-soft);
  --home-icon-bg: rgba(59, 130, 246, 0.1);
  --home-icon-color: #60a5fa;
  --home-hover-border: #3b82f6;
  --home-title: var(--vp-c-text-1);
  --home-accent: #60a5fa;
}

.home-hero {
  position: relative;
  padding: 64px 0 40px;
  text-align: center;
  isolation: isolate;
}

.home-hero::before {
  position: absolute;
  z-index: -1;
  inset: 0;
  background: radial-gradient(ellipse at center, rgba(59, 130, 246, 0.055), transparent 70%);
  content: '';
  pointer-events: none;
}

.home-eyebrow {
  margin: 0 0 12px;
  color: var(--vp-c-text-2);
  font-size: 15px;
  font-weight: 500;
  line-height: 24px;
}

.home-hero h1 {
  margin: 0;
  color: var(--home-title);
  font-size: clamp(32px, 5vw, 56px);
  font-weight: 750;
  letter-spacing: -1.5px;
  line-height: 1.2;
  text-wrap: balance;
}

.home-hero h1 span {
  color: var(--home-accent);
}

.home-description {
  margin: 18px 0 0;
  color: var(--vp-c-text-2);
  font-size: 18px;
  line-height: 1.7;
  text-wrap: balance;
}

.home-actions {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 16px;
  margin-top: 28px;
}

.home-button {
  display: inline-flex;
  min-height: 46px;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 10px 26px;
  border: 1px solid transparent;
  border-radius: 8px;
  font-size: 15px;
  font-weight: 600;
  line-height: 24px;
  transition: background-color 0.2s, border-color 0.2s;
}

.primary {
  border-color: var(--vp-c-brand-3);
  background: var(--vp-c-brand-3);
  color: #fff;
}

.primary:hover {
  border-color: #1d4ed8;
  background: #1d4ed8;
}

.secondary {
  border-color: var(--home-hover-border);
  background: var(--home-card-bg);
  color: var(--home-accent);
}

.secondary:hover {
  background: var(--home-icon-bg);
  border-color: var(--home-accent);
}

.docs-home svg {
  width: 22px;
  height: 22px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.home-guides {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
}

.guide-card,
.preview-card {
  border: 1px solid var(--home-border);
  border-radius: 12px;
  background: var(--home-card-bg);
  transition: border-color 0.2s, background-color 0.2s;
}

.guide-card:hover,
.preview-card:hover {
  border-color: var(--home-hover-border);
}

.home-button:focus-visible,
.guide-card:focus-visible,
.preview-card:focus-visible {
  outline: 3px solid var(--home-accent);
  outline-offset: 4px;
}

.guide-card {
  padding: 22px 24px;
}

.guide-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.guide-icon {
  display: flex;
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: var(--home-icon-bg);
  color: var(--home-icon-color);
}

.guide-icon svg {
  width: 25px;
  height: 25px;
}

.guide-arrow {
  color: var(--vp-c-text-3);
}

.guide-card h2 {
  margin: 0;
  color: var(--home-title);
  font-size: 19px;
  font-weight: 650;
  line-height: 28px;
}

.guide-card p {
  margin: 6px 0 0;
  color: var(--vp-c-text-2);
  font-size: 14px;
  line-height: 24px;
}

.home-previews {
  margin-top: 36px;
  padding-top: 28px;
  border-top: 1px solid var(--home-border);
}

.home-previews > h2 {
  margin: 0 0 18px;
  color: var(--home-title);
  font-size: 20px;
  font-weight: 650;
  line-height: 28px;
}

.preview-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.preview-card {
  min-width: 0;
  padding: 8px;
}

.preview-image {
  overflow: hidden;
  aspect-ratio: 16 / 10;
  border: 1px solid var(--home-border);
  border-radius: 6px;
  background: var(--vp-c-bg-soft);
}

.preview-image img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: top left;
}

.preview-light {
  display: block;
}

.preview-dark {
  display: none;
}

/* Match VitePress's theme before hydration, including saved dark-mode visits. */
:global(.dark .docs-home .preview-light) {
  display: none;
}

:global(.dark .docs-home .preview-dark) {
  display: block;
}

.preview-card h3 {
  margin: 10px 4px 4px;
  color: var(--home-title);
  font-size: 14px;
  font-weight: 600;
  line-height: 24px;
}

@media (max-width: 959px) {
  .docs-home {
    padding: 0 24px 40px;
  }

  .home-hero {
    padding-top: 48px;
  }

  .home-guides {
    gap: 14px;
  }

  .guide-card {
    padding: 20px;
  }

  .preview-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 639px) {
  .docs-home {
    padding: 0 20px 32px;
  }

  .home-hero {
    padding: 36px 0 32px;
  }

  .home-hero h1 {
    letter-spacing: -0.8px;
  }

  .home-description {
    margin-top: 14px;
    font-size: 16px;
  }

  .home-actions {
    gap: 12px;
    margin-top: 22px;
  }

  .home-button {
    padding-inline: 20px;
  }

  .home-guides {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .guide-card {
    display: grid;
    grid-template-columns: 44px minmax(0, 1fr);
    gap: 2px 16px;
    padding: 18px;
  }

  .guide-top {
    grid-row: 1 / 3;
    margin: 0;
  }

  .guide-arrow {
    display: none;
  }

  .guide-card p {
    margin: 0;
  }

  .home-previews {
    margin-top: 28px;
    padding-top: 24px;
  }

  .preview-grid {
    gap: 12px;
  }

  .preview-card {
    padding: 6px;
  }

  .preview-card h3 {
    margin-inline: 2px;
    font-size: 13px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .home-button,
  .guide-card,
  .preview-card {
    transition: none;
  }
}
</style>
