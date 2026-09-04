<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useReaderStore, type ReaderFeatureKey } from '@/stores/reader'
import type { SidebarItem } from '@/types/pdf'
import ProjectPanel from '@/components/ProjectPanel.vue'
import ProviderPanel from '@/components/ProviderPanel.vue'
import TranslationPanel from '@/components/TranslationPanel.vue'
import { getRuntimeInfo, openRuntimePath, selectRuntimeCacheDir } from '@/api/runtime'
import type { RuntimeInfo } from '@/api/types'

const store = useReaderStore()
const SIDEBAR_WIDTH_KEY = 'funpdf.sidebarPanelWidth'
const sidebarWidth = ref(initialSidebarWidth())
const resizing = ref(false)
const runtimeInfo = ref<RuntimeInfo | null>(null)
const runtimeInfoError = ref('')
const runtimePathMessage = ref('')
const changingCacheDir = ref(false)

const items: SidebarItem[] = [
  { id: 'albums', label: '项目', icon: 'fa-regular fa-folder-open' },
  { id: 'pages', label: '页面', icon: 'fa-regular fa-file-lines' },
  { id: 'annotations', label: '标注', icon: 'fa-regular fa-comment-dots' },
  { id: 'translation', label: '翻译', icon: 'fa-solid fa-language' },
  { id: 'ai', label: 'AI', icon: 'fa-solid fa-wand-magic-sparkles' },
]
const settingsItem: SidebarItem = { id: 'settings', label: '设置', icon: 'fa-solid fa-gear' }
const featureOptions: Array<{ key: ReaderFeatureKey; label: string; description: string; icon: string }> = [
  { key: 'aiChat', label: 'AI Chat', description: '顶部 AI 按钮、右侧对话栏、选区问 AI、AI 服务商入口', icon: 'fa-regular fa-comments' },
  { key: 'translation', label: 'Translator', description: '翻译配置页、选区翻译、翻译弹窗', icon: 'fa-solid fa-language' },
  { key: 'notes', label: '便签', description: '便签工具、选区便签、便签侧栏和页面便签标记', icon: 'fa-regular fa-note-sticky' },
]
const visibleItems = computed(() => items.filter(item => {
  if (item.id === 'ai') return store.featureFlags.aiChat
  if (item.id === 'translation') return store.featureFlags.translation
  if (item.id === 'annotations') return store.featureFlags.notes
  return true
}))
const allItems = computed(() => [...visibleItems.value, settingsItem])

function selectItem(id: string) {
  if (store.activeSidebar === id && store.sidebarOpen) store.setSidebarOpen(false)
  else store.setActiveSidebar(id)
}

function initialSidebarWidth() {
  const fallback = 300
  if (typeof window === 'undefined') return fallback
  const saved = Number(window.localStorage.getItem(SIDEBAR_WIDTH_KEY))
  return normalizeSidebarWidth(Number.isFinite(saved) && saved > 0 ? saved : fallback)
}

function normalizeSidebarWidth(width: number) {
  if (typeof window === 'undefined') return Math.min(Math.max(width, 260), 520)
  const max = Math.max(Math.min(window.innerWidth - 160, 560), 260)
  return Math.min(Math.max(width, 260), max)
}

const sidebarPanelStyle = computed(() => ({ width: `${sidebarWidth.value}px` }))

function startResize(event: PointerEvent) {
  const target = event.currentTarget as HTMLElement
  target.setPointerCapture(event.pointerId)
  resizing.value = true
}

function moveResize(event: PointerEvent) {
  if (!resizing.value) return
  const shellRect = (event.currentTarget as HTMLElement).closest('.sidebar-shell')?.getBoundingClientRect()
  if (!shellRect) return
  sidebarWidth.value = normalizeSidebarWidth(event.clientX - shellRect.left - 48)
}

function endResize(event: PointerEvent) {
  if (!resizing.value) return
  resizing.value = false
  const target = event.currentTarget as HTMLElement
  if (target.hasPointerCapture(event.pointerId)) target.releasePointerCapture(event.pointerId)
  try {
    window.localStorage.setItem(SIDEBAR_WIDTH_KEY, String(sidebarWidth.value))
  } catch {
    // Ignore unavailable storage.
  }
}

function handleSidebarWheel(event: WheelEvent) {
  const target = event.target as HTMLElement | null
  const content = target?.closest('.panel-content') as HTMLElement | null
  if (!content) return

  const maxScroll = content.scrollHeight - content.clientHeight
  if (maxScroll <= 0) return

  const nextTop = Math.min(Math.max(content.scrollTop + event.deltaY, 0), maxScroll)
  if (nextTop === content.scrollTop) return

  event.preventDefault()
  event.stopPropagation()
  content.scrollTop = nextTop
}

function focusAnnotation(annotationId: string, page: number) {
  store.currentPage = page
  window.dispatchEvent(new CustomEvent('funpdf:focus-annotation', {
    detail: { annotationId, page },
  }))
}

function deleteNote(annotationId: string, page: number) {
  window.dispatchEvent(new CustomEvent('funpdf:delete-note', {
    detail: { annotationId, page },
  }))
}

async function loadRuntimeInfo() {
  try {
    runtimeInfo.value = await getRuntimeInfo()
    runtimeInfoError.value = ''
    runtimePathMessage.value = ''
  } catch (error) {
    runtimeInfoError.value = error instanceof Error ? error.message : 'Failed to load runtime info'
  }
}

async function openPath(path?: string) {
  if (!path) return
  runtimePathMessage.value = ''

  try {
    await openRuntimePath(path)
    runtimePathMessage.value = 'Opened in Explorer.'
  } catch {
    try {
      await navigator.clipboard.writeText(path)
      runtimePathMessage.value = 'Path copied. Open-path API is not available yet.'
    } catch {
      runtimePathMessage.value = 'Open-path API is not available yet.'
    }
  }
}

async function changeCacheDir() {
  if (changingCacheDir.value) return
  changingCacheDir.value = true
  runtimePathMessage.value = ''
  runtimeInfoError.value = ''

  try {
    const result = await selectRuntimeCacheDir()
    if ('mode' in result) {
      runtimeInfo.value = result
    } else if (runtimeInfo.value) {
      runtimeInfo.value = { ...runtimeInfo.value, cache_dir: result.cache_dir }
    } else {
      runtimeInfo.value = await getRuntimeInfo()
    }
    runtimePathMessage.value = 'Cache location updated.'
    window.dispatchEvent(new Event('funpdf:files-changed'))
  } catch (error) {
    runtimeInfoError.value = error instanceof Error ? error.message : 'Failed to change cache location'
  } finally {
    changingCacheDir.value = false
  }
}

onMounted(() => {
  void loadRuntimeInfo()
})
</script>

<template>
  <aside class="sidebar-shell">
    <button class="rail-brand" title="FunPDF" @click="selectItem('albums')">
      <img src="/FunPDF.png" alt="FunPDF" />
    </button>
    <nav class="rail" aria-label="文档导航">
      <button
        v-for="item in visibleItems"
        :key="item.id"
        class="rail-button"
        :class="{ active: store.activeSidebar === item.id && store.sidebarOpen }"
        :title="item.label"
        @click="selectItem(item.id)"
      >
        <i :class="item.icon"></i>
        <span v-if="item.id === 'annotations' && store.noteCount" class="count-badge">
          {{ store.noteCount > 99 ? '99+' : store.noteCount }}
        </span>
      </button>
      <button
        class="rail-button rail-settings"
        :class="{ active: store.activeSidebar === 'settings' && store.sidebarOpen }"
        title="设置"
        @click="selectItem('settings')"
      >
        <i class="fa-solid fa-gear"></i>
      </button>
    </nav>

    <transition name="sidebar">
      <section v-if="store.sidebarOpen" class="sidebar-panel" :style="sidebarPanelStyle" @wheel="handleSidebarWheel">
        <div class="panel-header">
          <div>
            <div class="panel-title">{{ allItems.find(item => item.id === store.activeSidebar)?.label }}</div>
            <div class="panel-subtitle">{{ store.documentName || 'FunPDF' }}</div>
          </div>
          <button class="close-button" title="关闭" @click="store.setSidebarOpen(false)">
            <i class="fa-solid fa-xmark"></i>
          </button>
        </div>

        <div v-if="store.activeSidebar === 'albums'" class="panel-content">
          <ProjectPanel />
        </div>

        <div v-else-if="store.activeSidebar === 'pages'" class="panel-content">
          <div v-if="store.totalPages === 0" class="empty">打开 PDF 后，这里会显示页面列表。</div>
          <button
            v-for="page in store.totalPages"
            v-else
            :key="page"
            class="page-item"
            :class="{ active: page === store.currentPage }"
            @click="store.currentPage = page"
          >
            <div class="page-preview">
              <img
                v-if="store.pageThumbnails[page]"
                :src="store.pageThumbnails[page]"
                :alt="`第 ${page} 页缩略图`"
              />
              <i v-else class="fa-solid fa-circle-notch fa-spin"></i>
            </div>
            <span>第 {{ page }} 页</span>
          </button>
        </div>

        <div v-else-if="store.activeSidebar === 'annotations' && store.featureFlags.notes" class="panel-content">
          <div class="annotation-summary">
            <strong>{{ store.noteCount }}</strong>
            <span>条便签</span>
          </div>
          <div v-if="store.noteCount === 0" class="empty">选中文字后点击便签，或直接使用便签工具在页面上添加独立便签。</div>
          <template v-else>
            <h3 class="comment-group-title">批注</h3>
            <article
              v-for="comment in store.noteComments.filter(item => item.text)"
              :key="comment.id"
              class="comment-item"
              role="button"
              tabindex="0"
              @click="focusAnnotation(comment.id, comment.page)"
              @keyup.enter="focusAnnotation(comment.id, comment.page)"
            >
              <span class="comment-meta">
                <span>第 {{ comment.page }} 页</span>
                <button title="删除便签" @click.stop="deleteNote(comment.id, comment.page)" @keyup.enter.stop>
                  <i class="fa-regular fa-trash-can"></i>
                </button>
              </span>
              <strong>{{ comment.text }}</strong>
              <small v-if="comment.quoteText">{{ comment.quoteText }}</small>
            </article>
            <div v-if="!store.noteComments.some(item => item.text)" class="empty compact-empty">暂无批注便签。</div>

            <h3 class="comment-group-title">翻译</h3>
            <template v-for="comment in store.noteComments" :key="`${comment.id}-translations`">
              <article
                v-for="translation in comment.translations ?? []"
                :key="translation.id"
                class="comment-item translation-item"
                role="button"
                tabindex="0"
                @click="focusAnnotation(comment.id, comment.page)"
                @keyup.enter="focusAnnotation(comment.id, comment.page)"
              >
                <span class="comment-meta">
                  <span>第 {{ comment.page }} 页</span>
                  <button title="删除便签" @click.stop="deleteNote(comment.id, comment.page)" @keyup.enter.stop>
                    <i class="fa-regular fa-trash-can"></i>
                  </button>
                </span>
                <strong>{{ translation.translatedText }}</strong>
                <small>{{ translation.sourceText }}</small>
              </article>
            </template>
            <div v-if="!store.noteComments.some(item => item.translations?.length)" class="empty compact-empty">暂无翻译便签。</div>
          </template>
        </div>

        <div v-else-if="store.activeSidebar === 'translation' && store.featureFlags.translation" class="panel-content">
          <TranslationPanel />
        </div>

        <div v-else-if="store.activeSidebar === 'ai' && store.featureFlags.aiChat" class="panel-content">
          <ProviderPanel />
          <div class="feature-card"><i class="fa-solid fa-wand-magic-sparkles"></i><div><strong>AI 功能</strong><p>解释、摘要、问答和论文检索将在后续版本中提供。</p></div></div>
        </div>

        <div v-else class="panel-content">
          <section class="settings-panel">
            <article v-for="feature in featureOptions" :key="feature.key" class="setting-row">
              <i :class="feature.icon"></i>
              <div>
                <strong>{{ feature.label }}</strong>
                <p>{{ feature.description }}</p>
              </div>
              <button
                class="switch"
                :class="{ enabled: store.featureFlags[feature.key] }"
                :aria-pressed="store.featureFlags[feature.key]"
                @click="store.setFeatureEnabled(feature.key, !store.featureFlags[feature.key])"
              >
                <span></span>
              </button>
            </article>
            <article class="runtime-card">
              <div class="runtime-title">
                <i class="fa-solid fa-circle-info"></i>
                <strong>Runtime</strong>
                <button class="runtime-refresh" title="Refresh" @click="loadRuntimeInfo">
                  <i class="fa-solid fa-rotate-right"></i>
                </button>
              </div>
              <div v-if="runtimeInfo" class="runtime-list">
                <div><span>Mode</span><code>{{ runtimeInfo.mode }}</code></div>
                <div><span>Version</span><code>{{ runtimeInfo.version }}</code></div>
                <div><span>Database</span><code>{{ runtimeInfo.database }}</code></div>
                <div class="runtime-path-row">
                  <span>Cache</span>
                  <code>{{ runtimeInfo.cache_dir }}</code>
                  <button class="runtime-open" :disabled="changingCacheDir" title="选择新的缓存目录" @click="changeCacheDir">
                    {{ changingCacheDir ? '...' : '更改' }}
                  </button>
                  <button class="runtime-open" title="在资源管理器中打开当前缓存目录" @click="openPath(runtimeInfo.cache_dir)">Open</button>
                </div>
              </div>
              <p v-if="runtimePathMessage" class="runtime-message">{{ runtimePathMessage }}</p>
              <p v-if="runtimeInfoError" class="runtime-error">{{ runtimeInfoError }}</p>
              <p v-else-if="!runtimeInfo" class="runtime-loading">Loading runtime info...</p>
            </article>
          </section>
        </div>

        <div
          class="resize-handle"
          title="拖动调整侧栏宽度"
          @pointerdown.prevent="startResize"
          @pointermove.prevent="moveResize"
          @pointerup.prevent="endResize"
          @pointercancel.prevent="endResize"
        ></div>
      </section>
    </transition>
  </aside>
</template>

<style scoped>
.sidebar-shell { height: 100%; min-height: 0; display: flex; background: #f6f6f6; border-right: 1px solid #dedede; position: relative; z-index: 10; }
.rail { width: 48px; flex: 0 0 48px; display: flex; flex-direction: column; align-items: center; padding-top: 50px; gap: 4px; background: #f3f3f3; }
.rail-button, .rail-brand, .close-button { border: 0; background: transparent; color: #62676d; cursor: pointer; }
.rail-brand { position: absolute; left: 6px; top: 8px; width: 36px; height: 36px; border-radius: 8px; display: grid; place-items: center; padding: 4px; z-index: 1; }
.rail-brand:hover { background: #e8e8e8; }
.rail-brand img { width: 100%; height: 100%; display: block; object-fit: contain; border-radius: 6px; }
.rail-button { width: 36px; height: 36px; border-radius: 7px; display: grid; place-items: center; position: relative; }
.rail-button:hover { background: #e8e8e8; }
.rail-button.active { background: #dedede; color: #262a2f; }
.rail-button.active::before { content: ''; position: absolute; left: -6px; top: 8px; width: 3px; height: 20px; border-radius: 4px; background: #5f6873; }
.rail-settings { margin-top: auto; margin-bottom: 8px; }
.count-badge { position: absolute; right: -3px; top: -2px; min-width: 16px; height: 16px; padding: 0 3px; display: grid; place-items: center; border-radius: 8px; background: #555b62; color: white; font-size: 9px; }
.sidebar-panel { height: 100%; min-height: 0; background: #fafafa; display: flex; flex-direction: column; border-left: 1px solid #ececec; overflow: hidden; position: relative; }
.resize-handle { position: absolute; top: 0; right: 0; bottom: 0; width: 7px; cursor: col-resize; z-index: 2; }
.resize-handle:hover { background: rgb(90 96 104 / 10%); }
.panel-header { height: 60px; flex: 0 0 60px; padding: 0 14px 0 16px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #e5e9ee; }
.panel-title { font-size: 14px; font-weight: 700; color: #30343a; }
.panel-subtitle { width: 190px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-top: 2px; font-size: 11px; color: #94a3b8; }
.close-button { width: 30px; height: 30px; border-radius: 6px; }
.close-button:hover { background: #e9eef3; }
.panel-content { flex: 1 1 auto; min-height: 0; padding: 12px; overflow: auto; }
.panel-content::-webkit-scrollbar-thumb { background: transparent; }
.panel-content:hover::-webkit-scrollbar-thumb { background: #c5c5c5; }
.panel-content { scrollbar-color: transparent transparent; }
.panel-content:hover { scrollbar-color: #c5c5c5 transparent; }
.empty { padding: 18px 12px; font-size: 13px; line-height: 1.7; color: #8491a3; text-align: center; }
.page-item { width: 100%; border: 0; background: transparent; border-radius: 8px; padding: 8px; display: flex; align-items: center; gap: 10px; cursor: pointer; color: #64686e; text-align: left; }
.page-item:hover, .page-item.active { background: #e9eef5; }
.page-item.active { color: #353a40; }
.page-preview { width: 72px; height: 94px; flex: 0 0 72px; display: grid; place-items: center; overflow: hidden; background: white; border: 1px solid #d6d6d6; border-radius: 3px; box-shadow: 0 1px 3px rgb(0 0 0 / 8%); color: #aaaeb3; }
.page-preview img { display: block; width: 100%; height: 100%; object-fit: contain; background: white; }
.annotation-summary { padding: 14px; border-radius: 9px; background: #ededed; color: #41464d; display: flex; align-items: baseline; gap: 6px; }
.annotation-summary strong { font-size: 24px; }
.annotation-summary span { font-size: 12px; }
.comment-item { width: 100%; margin-top: 10px; padding: 11px; border: 1px solid #e1e1e1; border-radius: 9px; background: #f8f8f8; color: #3f4449; cursor: pointer; text-align: left; }
.comment-item:hover { background: #eeeeee; }
.comment-item span { display: block; margin-bottom: 6px; color: #858a90; font-size: 11px; }
.comment-item .comment-meta { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 6px; }
.comment-item .comment-meta span { margin: 0; }
.comment-item .comment-meta button { width: 24px; height: 24px; flex: 0 0 24px; display: grid; place-items: center; border: 0; border-radius: 6px; background: transparent; color: #9b5d5d; cursor: pointer; }
.comment-item .comment-meta button:hover { background: #f1dddd; color: #8c3838; }
.comment-item strong { display: -webkit-box; overflow: hidden; color: #353a40; font-size: 13px; line-height: 1.45; -webkit-line-clamp: 3; -webkit-box-orient: vertical; }
.comment-item small { display: -webkit-box; overflow: hidden; margin-top: 7px; padding-top: 7px; border-top: 1px solid #e4e4e4; color: #8a6d21; font-size: 11px; line-height: 1.45; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }
.comment-group-title { margin: 14px 0 4px; color: #555b62; font-size: 12px; }
.compact-empty { padding: 10px 8px; font-size: 12px; }
.translation-item strong { color: #315b72; }
.tip-card { margin-top: 10px; padding: 12px; border-radius: 8px; background: #f0f0f0; color: #676c72; font-size: 12px; line-height: 1.6; }
.tip-card i { color: #666c73; margin-right: 5px; }
.feature-card { padding: 14px; border-radius: 9px; border: 1px solid #e2e2e2; background: #f7f7f7; display: flex; gap: 12px; color: #565b61; }
.feature-card i { margin-top: 3px; }
.feature-card strong { display: block; margin-bottom: 5px; color: #35393e; font-size: 13px; }
.feature-card p { margin: 0; font-size: 12px; line-height: 1.6; color: #8491a3; }
.settings-panel { display: grid; gap: 10px; }
.setting-row { display: grid; grid-template-columns: 24px minmax(0, 1fr) auto; align-items: center; gap: 10px; padding: 12px; border: 1px solid #e2e2e2; border-radius: 9px; background: #f7f7f7; color: #555b62; }
.setting-row > i { color: #656b72; text-align: center; }
.setting-row strong { display: block; color: #34383d; font-size: 13px; }
.setting-row p { margin: 4px 0 0; color: #858b92; font-size: 11px; line-height: 1.45; }
.runtime-card { display: grid; gap: 10px; padding: 12px; border: 1px solid #e2e2e2; border-radius: 9px; background: #f7f7f7; color: #555b62; font-size: 13px; }
.runtime-title { display: flex; align-items: center; gap: 8px; color: #34383d; font-size: 13px; }
.runtime-title i { color: #656b72; }
.runtime-refresh { margin-left: auto; border: 0; background: transparent; color: #656b72; cursor: pointer; }
.runtime-refresh:hover { color: #34383d; }
.runtime-list { display: grid; gap: 7px; }
.runtime-list div { display: grid; gap: 3px; }
.runtime-list span { color: #858b92; font-size: 11px; text-transform: uppercase; letter-spacing: .04em; }
.runtime-list code { display: block; overflow-wrap: anywhere; border-radius: 5px; padding: 6px; background: #ededed; color: #34383d; font-size: 11px; line-height: 1.45; }
.runtime-path-row { grid-template-columns: minmax(0, 1fr) auto auto; align-items: end; }
.runtime-path-row span { grid-column: 1 / -1; }
.runtime-open { height: 28px; margin-left: 6px; padding: 0 9px; border: 1px solid #d8d8d8; border-radius: 6px; background: #eeeeee; color: #454a50; font-size: 11px; cursor: pointer; }
.runtime-open:hover { background: #e5e5e5; }
.runtime-open:disabled { opacity: .55; cursor: default; }
.runtime-error, .runtime-loading, .runtime-message { margin: 0; color: #858b92; font-size: 11px; line-height: 1.45; }
.switch { width: 38px; height: 22px; padding: 2px; border: 0; border-radius: 999px; background: #cfd3d7; cursor: pointer; }
.switch span { display: block; width: 18px; height: 18px; border-radius: 50%; background: white; box-shadow: 0 1px 3px rgb(0 0 0 / 18%); transition: transform .15s ease; }
.switch.enabled { background: #4f8f5d; }
.switch.enabled span { transform: translateX(16px); }
.sidebar-enter-active, .sidebar-leave-active { transition: width 0.18s ease, opacity 0.18s ease; }
.sidebar-enter-from, .sidebar-leave-to { width: 0; opacity: 0; }
@media (max-width: 720px) { .sidebar-panel { position: absolute; left: 48px; top: 0; bottom: 0; box-shadow: 8px 0 24px rgb(15 23 42 / 12%); } }
</style>
