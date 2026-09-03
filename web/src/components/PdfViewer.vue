p<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import * as pdfjsLib from 'pdfjs-dist'
import PdfWorker from 'pdfjs-dist/build/pdf.worker.min.mjs?url'
import { PDFDocument, rgb } from 'pdf-lib'
import type { PDFDocumentProxy } from 'pdfjs-dist'
import { useReaderStore } from '@/stores/reader'
import type { NoteTranslation, PdfAnnotation, PdfPoint, PdfRect } from '@/types/pdf'
import { FUNPDF_EDITOR_STATE_FORMAT, FUNPDF_PROJECT_VERSION, type FunPdfEditorState } from '@/types/project'
import { parseProjectText, type ProjectEditorState } from '@/utils/projectFile'
import {
  cachePdfFile,
  getCachedEditorState,
  getCachedFileContent,
  importLocalPdfPath,
  saveEditorState,
  deleteFileCache,
  type CachedFile,
} from '@/api/files'
import { apiErrorMessage } from '@/api/http'
import { completeTranslation, normalizeTranslatorName } from '@/api/translators'
import { onDesktopFileDrop } from '@/desktop/runtime'

pdfjsLib.GlobalWorkerOptions.workerSrc = PdfWorker

type AnnotationMap = Record<number, PdfAnnotation[]>
type PdfPage = Awaited<ReturnType<PDFDocumentProxy['getPage']>>
type PageViewport = ReturnType<PdfPage['getViewport']>
type RenderTask = ReturnType<PdfPage['render']>
type TextLayerInstance = InstanceType<typeof pdfjsLib.TextLayer>
type PageLayout = { pageNumber: number; width: number; height: number }
type PageLink = {
  id: string
  left: number
  top: number
  width: number
  height: number
  url: string
  dest?: unknown
  action?: string
  title: string
}
type LocalFileHandle = {
  name: string
  getFile(): Promise<File>
}
type OpenFilePicker = (options: Record<string, unknown>) => Promise<LocalFileHandle[]>
type OpenDocumentTab = {
  id: string
  name: string
  bytes: Uint8Array
  annotations: AnnotationMap
  undoStack: AnnotationMap[]
  redoStack: AnnotationMap[]
  rotation: number
  scale: number
  currentPage: number
  totalPages: number
  dirty: boolean
  cachedFileId: string
  cachedFileRevision: number
  cachedFileSha256: string
  saving: boolean
  autosaveTimer: number
}

const AUTOSAVE_INTERVAL_MS = 30_000

const store = useReaderStore()
const stageRef = ref<HTMLElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const pdfDocument = shallowRef<PDFDocumentProxy | null>(null)
const openTabs = ref<OpenDocumentTab[]>([])
const activeTabId = ref('')
const pageLayouts = ref<PageLayout[]>([])
const pageLinks = ref<Record<number, PageLink[]>>({})
const loading = ref(false)
const saving = ref(false)
const exporting = ref(false)
const dragActive = ref(false)
const rotation = ref(0)
const renderRevision = ref(0)
const focusedAnnotationId = ref('')
const textSelection = ref({ open: false, page: 0, text: '', rects: [] as DOMRect[], left: 0, top: 0 })
const translationPopup = ref({
  open: false,
  page: 0,
  sourceText: '',
  result: '',
  error: '',
  loading: false,
  left: 0,
  top: 0,
  width: 270,
  point: { x: 0, y: 0 } as PdfPoint,
  quoteText: '',
  quoteRects: [] as PdfRect[],
  translator: '',
  targetLanguage: '',
})
const noteEditor = ref({
  open: false,
  page: 0,
  annotationId: '',
  point: { x: 0, y: 0 } as PdfPoint,
  text: '',
  quoteText: '',
  quoteRects: [] as PdfRect[],
  translations: [] as NoteTranslation[],
  left: 0,
  top: 0,
  dragging: false,
  dragOffsetX: 0,
  dragOffsetY: 0,
})

const pageElements = new Map<number, HTMLElement>()
const pageCanvases = new Map<number, HTMLCanvasElement>()
const annotationCanvases = new Map<number, HTMLCanvasElement>()
const textLayerElements = new Map<number, HTMLDivElement>()
const pageViewports = new Map<number, PageViewport>()
const renderTasks = new Map<number, RenderTask>()
const textLayers = new Map<number, TextLayerInstance>()
const renderedPages = new Set<number>()
const renderingPages = new Set<number>()

let pdfLoadingTask: ReturnType<typeof pdfjsLib.getDocument> | null = null
let originalBytes: Uint8Array | null = null
let annotations: AnnotationMap = {}
let undoStack: AnnotationMap[] = []
let redoStack: AnnotationMap[] = []
let drawingAnnotation: PdfAnnotation | null = null
let drawingPage = 0
let gestureHistorySaved = false
let renderGeneration = 0
let pageObserver: IntersectionObserver | null = null
let scrollFrame = 0
let pageChangeCameFromScroll = false
let thumbnailGeneration = 0
let cachedFileId = ''
let cachedFileRevision = 0
let cachedFileSha256 = ''
let stopDesktopFileDrop: (() => void) | null = null
let draggingMarker: { page: number; annotationId: string; pointerId: number; moved: boolean } | null = null

function setMapElement<T extends Element>(map: Map<number, T>, page: number, element: unknown) {
  if (element instanceof Element) map.set(page, element as T)
  else map.delete(page)
}

function setPageElement(element: unknown, page: number) { setMapElement(pageElements, page, element) }
function setPageCanvas(element: unknown, page: number) { setMapElement(pageCanvases, page, element) }
function setAnnotationCanvas(element: unknown, page: number) { setMapElement(annotationCanvases, page, element) }
function setTextLayer(element: unknown, page: number) { setMapElement(textLayerElements, page, element) }

function cloneAnnotations(source: AnnotationMap = annotations): AnnotationMap {
  return JSON.parse(JSON.stringify(source)) as AnnotationMap
}

function annotationsForPage(page: number) {
  if (!annotations[page]) annotations[page] = []
  return annotations[page]
}

function updateHistoryState() {
  store.canUndo = undoStack.length > 0
  store.canRedo = redoStack.length > 0
  store.annotationCount = Object.values(annotations).reduce((sum, page) => sum + page.length, 0)
  const notes = Object.values(annotations)
    .flat()
    .filter((annotation): annotation is Extract<PdfAnnotation, { type: 'note' }> => annotation.type === 'note')
    .sort((a, b) => a.page - b.page)
  store.noteCount = notes.length
  store.noteComments = notes.map(note => ({
    id: note.id,
    page: note.page,
    text: note.text,
    quoteText: note.quoteText,
    translations: note.translations?.map(item => ({
      id: item.id,
      sourceText: item.sourceText,
      translatedText: item.translatedText,
    })),
  }))
  renderRevision.value++
}

function pushHistory() {
  undoStack.push(cloneAnnotations())
  if (undoStack.length > 100) undoStack.shift()
  redoStack = []
  store.dirty = true
  updateHistoryState()
}

function makeId() {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

function setStatus(message: string) {
  store.statusMessage = message
  window.setTimeout(() => {
    if (store.statusMessage === message) store.statusMessage = ''
  }, 3200)
}

function activeTab() {
  return openTabs.value.find(tab => tab.id === activeTabId.value)
}

function cloneHistoryStack(stack: AnnotationMap[]) {
  return stack.map(item => cloneAnnotations(item))
}

function buildEditorStateFrom(tab: OpenDocumentTab): FunPdfEditorState {
  return {
    format: FUNPDF_EDITOR_STATE_FORMAT,
    version: FUNPDF_PROJECT_VERSION,
    saved_at: new Date().toISOString(),
    source: {
      name: tab.name,
      mime_type: 'application/pdf',
      sha256: tab.cachedFileSha256 || undefined,
    },
    editor: {
      annotations: cloneAnnotations(tab.annotations),
      rotation: tab.rotation,
      scale: tab.scale,
      current_page: tab.currentPage,
    },
  }
}

function snapshotActiveTab() {
  const tab = activeTab()
  if (!tab || !originalBytes) return
  tab.name = store.documentName
  tab.bytes = originalBytes
  tab.annotations = cloneAnnotations()
  tab.undoStack = cloneHistoryStack(undoStack)
  tab.redoStack = cloneHistoryStack(redoStack)
  tab.rotation = rotation.value
  tab.scale = store.scale
  tab.currentPage = store.currentPage
  tab.totalPages = store.totalPages
  tab.dirty = store.dirty
  tab.cachedFileId = cachedFileId
  tab.cachedFileRevision = cachedFileRevision
  tab.cachedFileSha256 = cachedFileSha256
}

function createTab(name: string, bytes: Uint8Array) {
  const tab: OpenDocumentTab = {
    id: makeId(),
    name,
    bytes,
    annotations: cloneAnnotations(),
    undoStack: cloneHistoryStack(undoStack),
    redoStack: cloneHistoryStack(redoStack),
    rotation: rotation.value,
    scale: store.scale,
    currentPage: store.currentPage,
    totalPages: store.totalPages,
    dirty: store.dirty,
    cachedFileId,
    cachedFileRevision,
    cachedFileSha256,
    saving: false,
    autosaveTimer: 0,
  }
  openTabs.value.push(tab)
  activeTabId.value = tab.id
  store.activeDocumentId = tab.id
  store.activeCachedFileId = tab.cachedFileId
  if (tab.cachedFileId) startAutosave(tab)
  return tab
}

function startAutosave(tab: OpenDocumentTab) {
  stopAutosave(tab)
  tab.autosaveTimer = window.setInterval(() => void autosaveTab(tab.id), AUTOSAVE_INTERVAL_MS)
}

function stopAutosave(tab: OpenDocumentTab) {
  if (!tab.autosaveTimer) return
  window.clearInterval(tab.autosaveTimer)
  tab.autosaveTimer = 0
}

async function autosaveTab(tabId: string) {
  if (tabId === activeTabId.value) {
    if (saving.value || exporting.value || !originalBytes) return
    if (cachedFileId && !store.dirty) return
    await saveProject({ quiet: true })
    snapshotActiveTab()
    return
  }

  const tab = openTabs.value.find(item => item.id === tabId)
  if (!tab || tab.saving) return
  if (tab.cachedFileId && !tab.dirty) return
  if (!tab.cachedFileId) return
  tab.saving = true
  try {
    const state = buildEditorStateFrom(tab)
    if (tab.cachedFileId) {
      const result = await saveEditorState(tab.cachedFileId, tab.cachedFileRevision, state)
      tab.cachedFileRevision = result.revision
    }
    tab.dirty = false
  } catch (error) {
    console.error(error)
  } finally {
    tab.saving = false
  }
}

async function activateTab(tabId: string) {
  if (tabId === activeTabId.value || loading.value || saving.value || exporting.value) return
  const tab = openTabs.value.find(item => item.id === tabId)
  if (!tab) return
  snapshotActiveTab()
  activeTabId.value = tab.id
  store.activeDocumentId = tab.id
  store.activeCachedFileId = tab.cachedFileId
  loading.value = true
  clearTextSelection(true)
  try {
    await openPdfBytes(tab.bytes, tab.name, {
      annotations: tab.annotations,
      rotation: tab.rotation,
      scale: tab.scale,
      currentPage: tab.currentPage,
    })
    undoStack = cloneHistoryStack(tab.undoStack)
    redoStack = cloneHistoryStack(tab.redoStack)
    cachedFileId = tab.cachedFileId
    cachedFileRevision = tab.cachedFileRevision
    cachedFileSha256 = tab.cachedFileSha256
    store.dirty = tab.dirty
    updateHistoryState()
  } catch (error) {
    console.error(error)
    setStatus('无法切换到该 PDF')
  } finally {
    loading.value = false
  }
}

async function closeTab(tabId: string) {
  if (tabId === activeTabId.value) {
    await closeDocument()
    return
  }

  const tab = openTabs.value.find(item => item.id === tabId)
  if (!tab) return
  if (!tab.cachedFileId && tab.dirty && !window.confirm('当前文档尚未保存，确定关闭吗？')) return
  if (tab.cachedFileId && tab.dirty) await autosaveTab(tab.id)
  stopAutosave(tab)
  openTabs.value = openTabs.value.filter(item => item.id !== tab.id)
  window.dispatchEvent(new CustomEvent('funpdf:document-closed', { detail: { documentId: tab.id, fileId: tab.cachedFileId } }))
  if (tab.cachedFileId) void deleteFileCache(tab.cachedFileId).catch(() => undefined)
}

async function openFileDialog() {
  const picker = (window as Window & { showOpenFilePicker?: OpenFilePicker }).showOpenFilePicker
  if (!picker) {
    fileInputRef.value?.click()
    return
  }
  try {
    const [handle] = await picker({
      multiple: false,
      types: [{
        description: 'PDF 或 FunPDF 工程',
        accept: {
          'application/pdf': ['.pdf'],
          'application/x-funpdf+json': ['.funpdf'],
        },
      }],
    })
    if (!handle) return
    await loadFile(await handle.getFile())
  } catch (error: any) {
    if (error?.name !== 'AbortError') {
      console.error(error)
      setStatus('无法打开文件')
    }
  }
}

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) await loadFile(file)
  input.value = ''
}

async function loadFile(file: File) {
  const lowerName = file.name.toLowerCase()
  if (!lowerName.endsWith('.pdf') && !lowerName.endsWith('.funpdf') && file.type !== 'application/pdf') {
    setStatus('请选择 PDF 或 FunPDF 工程文件')
    return
  }

  snapshotActiveTab()
  loading.value = true
  clearTextSelection(true)
  try {
    if (lowerName.endsWith('.funpdf')) {
      cachedFileId = ''
      cachedFileRevision = 0
      cachedFileSha256 = ''
      const { project, pdfBytes } = parseProjectText(await file.text())
      await openPdfBytes(pdfBytes, project.document.name, {
        annotations: project.editor.annotations,
        rotation: project.editor.rotation,
        scale: project.editor.scale,
        currentPage: project.editor.current_page,
      })
      createTab(project.document.name, pdfBytes)
      setStatus(`已打开可编辑工程 ${file.name}`)
    } else {
      cachedFileId = ''
      cachedFileRevision = 0
      cachedFileSha256 = ''
      const bytes = new Uint8Array(await file.arrayBuffer())
      await openPdfBytes(bytes, file.name)
      createTab(file.name, bytes)
      setStatus(`已打开 ${file.name}；按 Ctrl+S 存入本地文件库`)
    }
  } catch (error) {
    console.error(error)
    setStatus(lowerName.endsWith('.funpdf') ? '无法打开工程，文件可能已损坏或版本不受支持' : '无法打开这个 PDF，文件可能已损坏或受密码保护')
  } finally {
    loading.value = false
  }
}

async function openCachedFile(file: CachedFile) {
  const opened = openTabs.value.find(tab => tab.cachedFileId === file.id)
  if (opened) {
    await activateTab(opened.id)
    return
  }
  snapshotActiveTab()
  loading.value = true
  clearTextSelection(true)
  try {
    const [content, state] = await Promise.all([
      getCachedFileContent(file.id),
      getCachedEditorState(file.id),
    ])
    const bytes = new Uint8Array(content)
    await openPdfBytes(bytes, file.name, {
      annotations: state.editor.annotations,
      rotation: state.editor.rotation,
      scale: state.editor.scale,
      currentPage: state.editor.current_page,
    })
    cachedFileId = file.id
    cachedFileRevision = file.revision
    cachedFileSha256 = file.sha256
    createTab(file.name, bytes)
    setStatus(`已从公共文件区打开 ${file.name}`)
  } catch (error) {
    console.error(error)
    setStatus(apiErrorMessage(error, '无法打开公共文件，请检查后端内容与状态接口'))
  } finally {
    loading.value = false
  }
}

async function generateDocumentThumbnail() {
  if (!pdfDocument.value) return ''
  const page = await pdfDocument.value.getPage(1)
  const baseViewport = page.getViewport({ scale: 1, rotation: rotation.value })
  const targetWidth = 240
  const viewport = page.getViewport({
    scale: targetWidth / baseViewport.width,
    rotation: rotation.value,
  })
  const canvas = document.createElement('canvas')
  const pixelRatio = Math.min(window.devicePixelRatio || 1, 2)
  canvas.width = Math.max(Math.round(viewport.width * pixelRatio), 1)
  canvas.height = Math.max(Math.round(viewport.height * pixelRatio), 1)
  const context = canvas.getContext('2d')
  if (!context) return ''
  await page.render({
    canvas,
    canvasContext: context,
    viewport,
    transform: pixelRatio === 1 ? undefined : [pixelRatio, 0, 0, pixelRatio, 0, 0],
  }).promise

  context.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0)
  context.lineCap = 'round'
  context.lineJoin = 'round'
  for (const annotation of annotations[1] ?? []) {
    context.save()
    context.strokeStyle = annotation.color
    context.fillStyle = annotation.color
    context.lineWidth = Math.max(annotation.width * viewport.scale, 0.75)
    if (annotation.type === 'pen' && annotation.points.length > 1) {
      context.beginPath()
      annotation.points.forEach((value, index) => {
        const [x, y] = viewport.convertToViewportPoint(value.x, value.y)
        index === 0 ? context.moveTo(x, y) : context.lineTo(x, y)
      })
      context.stroke()
    } else if (annotation.type === 'highlight') {
      const [startX, startY] = viewport.convertToViewportPoint(annotation.start.x, annotation.start.y)
      const [endX, endY] = viewport.convertToViewportPoint(annotation.end.x, annotation.end.y)
      context.globalAlpha = 0.28
      context.fillRect(Math.min(startX, endX), Math.min(startY, endY), Math.abs(endX - startX), Math.abs(endY - startY))
    } else if (annotation.type === 'underline' || annotation.type === 'strike') {
      const [startX, startY] = viewport.convertToViewportPoint(annotation.start.x, annotation.start.y)
      const [endX, endY] = viewport.convertToViewportPoint(annotation.end.x, annotation.end.y)
      context.beginPath()
      context.moveTo(startX, startY)
      context.lineTo(endX, endY)
      context.stroke()
    } else if (annotation.type === 'note') {
      const [x, y] = viewport.convertToViewportPoint(annotation.point.x, annotation.point.y)
      const radius = Math.max(11 * viewport.scale, 4)
      context.beginPath()
      context.arc(x, y, radius, 0, Math.PI * 2)
      context.fill()
      context.fillStyle = '#fff'
      context.font = `bold ${Math.max(12 * viewport.scale, 5)}px sans-serif`
      context.textAlign = 'center'
      context.textBaseline = 'middle'
      context.fillText('!', x, y)
    }
    context.restore()
  }
  return canvas.toDataURL('image/jpeg', 0.78)
}

function resetOpenedDocument() {
  renderGeneration++
  cancelPageRendering()
  pageObserver?.disconnect()
  pageObserver = null
  void pdfLoadingTask?.destroy()
  pdfLoadingTask = null
  pdfDocument.value = null
  originalBytes = null
  annotations = {}
  undoStack = []
  redoStack = []
  pageLayouts.value = []
  pageLinks.value = {}
  pageElements.clear()
  pageCanvases.clear()
  annotationCanvases.clear()
  textLayerElements.clear()
  pageViewports.clear()
  cachedFileId = ''
  cachedFileRevision = 0
  cachedFileSha256 = ''
  store.resetDocumentState()
  store.documentName = ''
}

async function closeDocument() {
  if (!originalBytes || saving.value || exporting.value) return !originalBytes
  const closingTab = activeTab()
  if (cachedFileId) {
    try {
      const thumbnail = await generateDocumentThumbnail()
      if (!thumbnail || !await saveProject({ quiet: true, thumbnail })) return false
      window.dispatchEvent(new Event('funpdf:files-changed'))
    } catch (error) {
      console.error(error)
      setStatus(apiErrorMessage(error, '关闭文件前更新缩略图失败'))
      return false
    }
  } else if (store.dirty && !window.confirm('当前文档尚未保存，确定关闭吗？')) {
    return false
  }

  if (closingTab) {
    stopAutosave(closingTab)
    const closedIndex = openTabs.value.findIndex(tab => tab.id === closingTab.id)
    openTabs.value = openTabs.value.filter(tab => tab.id !== closingTab.id)
    window.dispatchEvent(new CustomEvent('funpdf:document-closed', { detail: { documentId: closingTab.id, fileId: closingTab.cachedFileId } }))
    if (closingTab.cachedFileId) void deleteFileCache(closingTab.cachedFileId).catch(() => undefined)
    const nextTab = openTabs.value[Math.max(0, Math.min(closedIndex, openTabs.value.length - 1))]
    if (nextTab) {
      resetOpenedDocument()
      activeTabId.value = ''
      store.activeDocumentId = ''
      store.activeCachedFileId = ''
      await activateTab(nextTab.id)
      return true
    }
  }

  resetOpenedDocument()
  activeTabId.value = ''
  store.activeDocumentId = ''
  store.activeCachedFileId = ''
  return true
}

function handleOpenCachedFile(event: Event) {
  const file = (event as CustomEvent<{ file?: CachedFile }>).detail?.file
  if (file) void openCachedFile(file)
}

async function openPdfBytes(bytes: Uint8Array, documentName: string, restored?: ProjectEditorState) {
    const nextTask = pdfjsLib.getDocument({ data: bytes.slice() })
    const nextDocument = await nextTask.promise
    await pdfLoadingTask?.destroy()
    pdfLoadingTask = nextTask
    pdfDocument.value = nextDocument
    originalBytes = bytes
    annotations = restored ? cloneAnnotations(restored.annotations) : {}
    undoStack = []
    redoStack = []
    rotation.value = restored?.rotation ?? 0
    store.resetDocumentState()
    store.documentName = documentName
    store.totalPages = nextDocument.numPages
    store.activeTool = 'cursor'
    store.currentPage = Math.min(Math.max(restored?.currentPage ?? 1, 1), nextDocument.numPages)
    if (restored) store.scale = Math.min(Math.max(restored.scale, 0.4), 3)
    updateHistoryState()
    store.dirty = false

    await nextTick()
    if (!restored) await setInitialFitWidth()
    await refreshPageFlow(true)
    void generateThumbnails()
}

function handleDragOver(event: DragEvent) {
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  dragActive.value = true
}

function handleDragLeave(event: DragEvent) {
  if (event.currentTarget === event.target) dragActive.value = false
}

async function handleDrop(event: DragEvent) {
  event.preventDefault()
  dragActive.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file) await loadFile(file)
}

async function handleDesktopFileDrop(paths: string[]) {
  const path = paths[0]
  if (!path || loading.value) return

  loading.value = true
  dragActive.value = false
  try {
    const cached = await importLocalPdfPath(path)
    window.dispatchEvent(new Event('funpdf:files-changed'))
    await openCachedFile(cached)
  } catch (error) {
    console.error(error)
    setStatus(apiErrorMessage(error, '桌面端本机路径导入接口尚未实现'))
  } finally {
    loading.value = false
  }
}

async function setInitialFitWidth() {
  if (!pdfDocument.value || !stageRef.value) return
  const page = await pdfDocument.value.getPage(1)
  const viewport = page.getViewport({ scale: 1, rotation: rotation.value })
  const stageStyle = window.getComputedStyle(stageRef.value)
  const horizontalPadding =
    Number.parseFloat(stageStyle.paddingLeft || '0') +
    Number.parseFloat(stageStyle.paddingRight || '0')
  const available = Math.max(stageRef.value.clientWidth - horizontalPadding, 260)
  store.scale = Math.min(Math.max(available / viewport.width, 0.4), 3)
}

function cancelPageRendering() {
  for (const task of renderTasks.values()) task.cancel()
  for (const layer of textLayers.values()) layer.cancel()
  renderTasks.clear()
  textLayers.clear()
  renderingPages.clear()
  renderedPages.clear()
}

async function refreshPageFlow(scrollToCurrent = false) {
  if (!pdfDocument.value || store.totalPages === 0) return
  const generation = ++renderGeneration
  cancelPageRendering()
  clearTextSelection(true)
  pageLinks.value = {}

  const layouts = await Promise.all(
    Array.from({ length: store.totalPages }, async (_, index) => {
      const pageNumber = index + 1
      const page = await pdfDocument.value!.getPage(pageNumber)
      const viewport = page.getViewport({ scale: store.scale, rotation: rotation.value })
      pageViewports.set(pageNumber, viewport)
      return { pageNumber, width: viewport.width, height: viewport.height }
    }),
  )
  if (generation !== renderGeneration) return
  pageLayouts.value = layouts
  await nextTick()
  setupPageObserver()
  await renderPage(store.currentPage, generation)
  renderVisiblePages()
  if (scrollToCurrent) await nextTick(() => scrollToPage(store.currentPage, false))
}

function setupPageObserver() {
  pageObserver?.disconnect()
  if (!stageRef.value || typeof IntersectionObserver === 'undefined') {
    void renderPage(store.currentPage)
    return
  }
  pageObserver = new IntersectionObserver(entries => {
    for (const entry of entries) {
      if (entry.isIntersecting) {
        const page = Number((entry.target as HTMLElement).dataset.page)
        if (page) void renderPage(page)
      }
    }
  }, { root: stageRef.value, rootMargin: '900px 0px', threshold: 0.01 })
  for (const element of pageElements.values()) pageObserver.observe(element)
}

function renderVisiblePages() {
  const stage = stageRef.value
  if (!stage) return
  const stageRect = stage.getBoundingClientRect()
  for (const [page, element] of pageElements) {
    const rect = element.getBoundingClientRect()
    if (rect.bottom >= stageRect.top - 900 && rect.top <= stageRect.bottom + 900) void renderPage(page)
  }
}

async function renderPage(pageNumber: number, generation = renderGeneration) {
  if (!pdfDocument.value || renderedPages.has(pageNumber) || renderingPages.has(pageNumber)) return
  const canvas = pageCanvases.get(pageNumber)
  const annotationCanvas = annotationCanvases.get(pageNumber)
  const textContainer = textLayerElements.get(pageNumber)
  const viewport = pageViewports.get(pageNumber)
  if (!canvas || !annotationCanvas || !textContainer || !viewport) return

  renderingPages.add(pageNumber)
  try {
    const page = await pdfDocument.value.getPage(pageNumber)
    if (generation !== renderGeneration) return
    sizeCanvas(canvas, viewport)
    const context = canvas.getContext('2d')
    if (!context) return
    const scaleX = canvas.width / viewport.width
    const scaleY = canvas.height / viewport.height
    const task = page.render({
      canvas,
      canvasContext: context,
      transform: scaleX !== 1 || scaleY !== 1 ? [scaleX, 0, 0, scaleY, 0, 0] : undefined,
      viewport,
    })
    renderTasks.set(pageNumber, task)
    await task.promise
    if (generation !== renderGeneration) return

    sizeCanvas(annotationCanvas, viewport)
    drawAnnotations(pageNumber)
    textContainer.replaceChildren()
    textContainer.style.width = `${viewport.width}px`
    textContainer.style.height = `${viewport.height}px`
    textContainer.style.setProperty('--total-scale-factor', `${viewport.scale}`)
    const layer = new pdfjsLib.TextLayer({
      textContentSource: page.streamTextContent({ includeMarkedContent: true }),
      container: textContainer,
      viewport,
    })
    textLayers.set(pageNumber, layer)
    await layer.render()
    if (generation !== renderGeneration) return
    await renderPageLinks(page, pageNumber, viewport)
    if (generation !== renderGeneration) return
    renderedPages.add(pageNumber)
    createThumbnailFromCanvas(pageNumber, canvas)
  } catch (error: any) {
    if (error?.name !== 'RenderingCancelledException') {
      console.error(error)
      setStatus(`第 ${pageNumber} 页渲染失败`)
    }
  } finally {
    renderTasks.delete(pageNumber)
    renderingPages.delete(pageNumber)
  }
}

function safeExternalUrl(value: unknown) {
  if (typeof value !== 'string' || !value) return ''
  try {
    const url = new URL(value, window.location.href)
    return ['http:', 'https:', 'mailto:', 'tel:'].includes(url.protocol) ? url.href : ''
  } catch {
    return ''
  }
}

async function renderPageLinks(page: PdfPage, pageNumber: number, viewport: PageViewport) {
  const pageAnnotations = await page.getAnnotations({ intent: 'display' })
  const links: PageLink[] = []
  for (const annotation of pageAnnotations) {
    if (annotation.subtype !== 'Link' || !Array.isArray(annotation.rect)) continue
    const firstCorner = viewport.convertToViewportPoint(annotation.rect[0], annotation.rect[1])
    const secondCorner = viewport.convertToViewportPoint(annotation.rect[2], annotation.rect[3])
    const left = Math.min(firstCorner[0], secondCorner[0])
    const top = Math.min(firstCorner[1], secondCorner[1])
    const width = Math.abs(secondCorner[0] - firstCorner[0])
    const height = Math.abs(secondCorner[1] - firstCorner[1])
    if (width <= 0 || height <= 0) continue
    const url = safeExternalUrl(annotation.url)
    links.push({
      id: annotation.id ?? `${pageNumber}-${links.length}`,
      left,
      top,
      width,
      height,
      url,
      dest: annotation.dest,
      action: annotation.action,
      title: url || annotation.action || '跳转到文档位置',
    })
  }
  pageLinks.value = { ...pageLinks.value, [pageNumber]: links }
}

async function activatePdfLink(link: PageLink, event: MouseEvent) {
  if (link.url) return
  event.preventDefault()

  if (link.action) {
    const actions: Record<string, number> = {
      FirstPage: 1,
      PrevPage: Math.max(store.currentPage - 1, 1),
      NextPage: Math.min(store.currentPage + 1, store.totalPages),
      LastPage: store.totalPages,
    }
    const targetPage = actions[link.action]
    if (targetPage) store.currentPage = targetPage
    return
  }

  if (!pdfDocument.value || !link.dest) return
  try {
    const destination = typeof link.dest === 'string'
      ? await pdfDocument.value.getDestination(link.dest)
      : link.dest
    if (!Array.isArray(destination) || destination.length === 0) return
    const target = destination[0]
    const pageIndex = typeof target === 'number'
      ? target
      : await pdfDocument.value.getPageIndex(target)
    store.currentPage = pageIndex + 1
  } catch (error) {
    console.error(error)
    setStatus('无法跳转到这个文档位置')
  }
}

function sizeCanvas(canvas: HTMLCanvasElement, viewport: PageViewport) {
  const scale = window.devicePixelRatio || 1
  canvas.width = Math.max(Math.round(viewport.width * scale), 1)
  canvas.height = Math.max(Math.round(viewport.height * scale), 1)
  canvas.style.width = `${viewport.width}px`
  canvas.style.height = `${viewport.height}px`
}

function createThumbnailFromCanvas(page: number, source: HTMLCanvasElement) {
  if (store.pageThumbnails[page]) return
  const width = 120
  const height = Math.max(Math.round(width * source.height / source.width), 1)
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  canvas.getContext('2d')?.drawImage(source, 0, 0, width, height)
  store.pageThumbnails = { ...store.pageThumbnails, [page]: canvas.toDataURL('image/jpeg', 0.76) }
}

async function generateThumbnails() {
  if (!pdfDocument.value) return
  const generation = ++thumbnailGeneration
  for (let pageNumber = 1; pageNumber <= store.totalPages; pageNumber++) {
    if (generation !== thumbnailGeneration || !pdfDocument.value) return
    if (store.pageThumbnails[pageNumber]) continue
    try {
      const page = await pdfDocument.value.getPage(pageNumber)
      const base = page.getViewport({ scale: 1, rotation: rotation.value })
      const viewport = page.getViewport({ scale: 120 / base.width, rotation: rotation.value })
      const canvas = document.createElement('canvas')
      canvas.width = Math.max(Math.round(viewport.width), 1)
      canvas.height = Math.max(Math.round(viewport.height), 1)
      await page.render({ canvas, viewport }).promise
      if (generation !== thumbnailGeneration) return
      store.pageThumbnails = { ...store.pageThumbnails, [pageNumber]: canvas.toDataURL('image/jpeg', 0.76) }
    } catch (error) {
      console.error(error)
    }
  }
}

function pageViewport(page: number) { return pageViewports.get(page) }

function viewportPoint(point: PdfPoint, page: number): PdfPoint {
  const viewport = pageViewport(page)
  if (!viewport) return point
  const [x, y] = viewport.convertToViewportPoint(point.x, point.y)
  return { x, y }
}

function pdfPoint(point: PdfPoint, page: number): PdfPoint {
  const viewport = pageViewport(page)
  if (!viewport) return point
  const [x, y] = viewport.convertToPdfPoint(point.x, point.y)
  return { x, y }
}

function drawAnnotations(page: number) {
  const canvas = annotationCanvases.get(page)
  const viewport = pageViewport(page)
  if (!canvas || !viewport) return
  const context = canvas.getContext('2d')
  if (!context) return
  context.setTransform(canvas.width / viewport.width, 0, 0, canvas.height / viewport.height, 0, 0)
  context.clearRect(0, 0, viewport.width, viewport.height)
  context.lineCap = 'round'
  context.lineJoin = 'round'

  for (const annotation of annotations[page] ?? []) {
    context.save()
    context.strokeStyle = annotation.color
    context.fillStyle = annotation.color
    context.lineWidth = Math.max(annotation.width * store.scale, 1)
    if (annotation.type === 'pen') {
      if (annotation.points.length > 1) {
        context.beginPath()
        annotation.points.forEach((point, index) => {
          const current = viewportPoint(point, page)
          index === 0 ? context.moveTo(current.x, current.y) : context.lineTo(current.x, current.y)
        })
        context.stroke()
      }
    } else if (annotation.type === 'highlight') {
      const start = viewportPoint(annotation.start, page)
      const end = viewportPoint(annotation.end, page)
      context.globalAlpha = 0.28
      context.fillRect(Math.min(start.x, end.x), Math.min(start.y, end.y), Math.abs(end.x - start.x), Math.abs(end.y - start.y))
    } else if (annotation.type === 'underline' || annotation.type === 'strike') {
      const start = viewportPoint(annotation.start, page)
      const end = viewportPoint(annotation.end, page)
      context.beginPath()
      context.moveTo(start.x, start.y)
      context.lineTo(end.x, end.y)
      context.stroke()
    } else if (annotation.type === 'note') {
      if (annotation.quoteRects?.length) {
        const quoteRects = annotation.quoteRects
        context.globalAlpha = annotation.translations?.length && !annotation.text ? 0.28 : 0.34
        context.fillStyle = annotation.translations?.length && !annotation.text ? '#2563eb' : annotation.color
        for (const quoteRect of quoteRects) {
          const start = viewportPoint(quoteRect.start, page)
          const end = viewportPoint(quoteRect.end, page)
          context.fillRect(Math.min(start.x, end.x), Math.max(start.y, end.y) - 2, Math.abs(end.x - start.x), 2)
        }
        context.globalAlpha = 1
        const lastRect = quoteRects[quoteRects.length - 1]
        const start = viewportPoint(lastRect.start, page)
        const end = viewportPoint(lastRect.end, page)
        const x = Math.min(Math.max(Math.max(start.x, end.x) + 3, 4), viewport.width - 4)
        context.fillRect(x, Math.min(start.y, end.y), 4, Math.max(Math.abs(end.y - start.y), 10))
        context.restore()
        continue
      }
      const point = viewportPoint(annotation.point, page)
      context.beginPath()
      context.arc(point.x, point.y, 11, 0, Math.PI * 2)
      context.fill()
      context.fillStyle = '#fff'
      context.font = 'bold 12px sans-serif'
      context.textAlign = 'center'
      context.textBaseline = 'middle'
      context.fillText(annotation.translations?.length && !annotation.text ? '…' : '!', point.x, point.y + 0.5)
    }
    context.restore()
  }
}

function redrawAnnotations() {
  for (const page of renderedPages) drawAnnotations(page)
  renderRevision.value++
}

function localPointerPosition(event: PointerEvent, page: number): PdfPoint {
  const canvas = annotationCanvases.get(page)!
  const viewport = pageViewport(page)!
  const rect = canvas.getBoundingClientRect()
  return {
    x: Math.min(Math.max((event.clientX - rect.left) * viewport.width / rect.width, 0), viewport.width),
    y: Math.min(Math.max((event.clientY - rect.top) * viewport.height / rect.height, 0), viewport.height),
  }
}

function setCurrentPageFromView(page: number) {
  if (store.currentPage === page) return
  pageChangeCameFromScroll = true
  store.currentPage = page
}

function startPointer(event: PointerEvent, page: number) {
  const canvas = annotationCanvases.get(page)
  if (!canvas || !pageViewport(page) || store.activeTool === 'cursor') return
  event.preventDefault()
  setCurrentPageFromView(page)
  canvas.setPointerCapture(event.pointerId)
  const local = localPointerPosition(event, page)
  const point = pdfPoint(local, page)
  drawingPage = page
  gestureHistorySaved = false

  if (store.activeTool === 'note') {
    if (!store.featureFlags.notes) return
    openNoteEditor(page, point, local)
    return
  }
  if (store.activeTool === 'eraser') {
    eraseAt(local, page)
    return
  }

  pushHistory()
  gestureHistorySaved = true
  drawingAnnotation = store.activeTool === 'pen'
    ? { id: makeId(), page, type: 'pen', color: store.annotationColor, width: store.annotationWidth / store.scale, points: [point] }
    : { id: makeId(), page, type: store.activeTool, color: store.annotationColor, width: store.annotationWidth / store.scale, start: point, end: point }
  annotationsForPage(page).push(drawingAnnotation as PdfAnnotation)
  drawAnnotations(page)
}

function movePointer(event: PointerEvent, page: number) {
  const canvas = annotationCanvases.get(page)
  if (!canvas?.hasPointerCapture(event.pointerId)) return
  const local = localPointerPosition(event, page)
  if (store.activeTool === 'eraser') return eraseAt(local, page)
  if (!drawingAnnotation || drawingPage !== page) return
  const point = pdfPoint(local, page)
  if (drawingAnnotation.type === 'pen') drawingAnnotation.points.push(point)
  else if ('end' in drawingAnnotation) drawingAnnotation.end = point
  drawAnnotations(page)
}

function endPointer(event: PointerEvent, page: number) {
  const canvas = annotationCanvases.get(page)
  if (canvas?.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId)
  if (drawingAnnotation?.type === 'pen' && drawingAnnotation.points.length === 1) {
    const first = drawingAnnotation.points[0]
    drawingAnnotation.points.push({ x: first.x + 0.1, y: first.y + 0.1 })
  }
  drawingAnnotation = null
  drawingPage = 0
  if (gestureHistorySaved) updateHistoryState()
  gestureHistorySaved = false
  drawAnnotations(page)
}

function distanceToSegment(point: PdfPoint, start: PdfPoint, end: PdfPoint) {
  const dx = end.x - start.x
  const dy = end.y - start.y
  if (!dx && !dy) return Math.hypot(point.x - start.x, point.y - start.y)
  const t = Math.max(0, Math.min(1, ((point.x - start.x) * dx + (point.y - start.y) * dy) / (dx * dx + dy * dy)))
  return Math.hypot(point.x - start.x - t * dx, point.y - start.y - t * dy)
}

function hitAnnotation(annotation: PdfAnnotation, point: PdfPoint, page: number) {
  if (annotation.type === 'note') {
    const note = viewportPoint(annotation.point, page)
    return Math.hypot(point.x - note.x, point.y - note.y) <= 16
  }
  if (annotation.type === 'highlight') {
    const start = viewportPoint(annotation.start, page)
    const end = viewportPoint(annotation.end, page)
    return point.x >= Math.min(start.x, end.x) - 5 && point.x <= Math.max(start.x, end.x) + 5
      && point.y >= Math.min(start.y, end.y) - 5 && point.y <= Math.max(start.y, end.y) + 5
  }
  if (annotation.type === 'underline' || annotation.type === 'strike') {
    return distanceToSegment(point, viewportPoint(annotation.start, page), viewportPoint(annotation.end, page)) <= 9
  }
  if (annotation.type === 'pen') {
    for (let index = 1; index < annotation.points.length; index++) {
      if (distanceToSegment(point, viewportPoint(annotation.points[index - 1], page), viewportPoint(annotation.points[index], page)) <= 10) return true
    }
  }
  return false
}

function eraseAt(point: PdfPoint, page: number) {
  const pageAnnotations = annotationsForPage(page)
  const reverseIndex = [...pageAnnotations].reverse().findIndex(annotation => hitAnnotation(annotation, point, page))
  if (reverseIndex < 0) return
  if (!gestureHistorySaved) { pushHistory(); gestureHistorySaved = true }
  pageAnnotations.splice(pageAnnotations.length - 1 - reverseIndex, 1)
  store.dirty = true
  updateHistoryState()
  drawAnnotations(page)
}

function noteMarkers(page: number) {
  renderRevision.value
  if (!store.featureFlags.notes) return []
  const viewport = pageViewport(page)
  if (!viewport) return []
  const leftRailX = -44
  const rightRailX = viewport.width + 14
  const maxRailTop = Math.max(viewport.height - 38, 10)
  const markers = (annotations[page] ?? [])
    .filter(annotation => annotation.type === 'note')
    .filter(annotation => store.featureFlags.translation || annotation.text || !annotation.translations?.length)
    .map(annotation => {
      const quoteRects = annotation.quoteRects ?? []
      const firstQuoteRect = quoteRects[0]
      const lastQuoteRect = quoteRects[quoteRects.length - 1]
      const start = firstQuoteRect ? viewportPoint(firstQuoteRect.start, page) : undefined
      const end = lastQuoteRect ? viewportPoint(lastQuoteRect.end, page) : undefined
      const point = start && end
        ? {
            x: Math.min(Math.max(end.x + 4, 8), Math.max(viewport.width - 8, 8)),
            y: Math.min(Math.max((start.y + end.y) / 2, 10), Math.max(viewport.height - 10, 10)),
          }
        : viewportPoint(annotation.point, page)
      const isTranslation = Boolean(annotation.translations?.length && !annotation.text)
      const railTop = Math.min(Math.max(point.y - 13, 10), Math.max(viewport.height - 38, 10))
      const railSide = quoteRects.length && point.x < viewport.width / 2 ? 'left' : 'right'
      const railLeft = railSide === 'left' ? leftRailX : rightRailX
      return {
        annotation,
        point,
        label: isTranslation ? 'T' : 'N',
        isTranslation,
        railSide,
        railLeft,
        railTop,
        connectorX1: point.x,
        connectorY1: point.y,
        connectorX2: railSide === 'left' ? railLeft + 30 : railLeft,
        connectorY2: railTop + 13,
        railOffset: 0,
      }
    })

  for (const railSide of ['left', 'right'] as const) {
    const railMarkers = markers
      .filter(marker => marker.annotation.quoteRects?.length && marker.railSide === railSide)
      .sort((a, b) => a.railTop - b.railTop)

    for (let index = 0; index < railMarkers.length; index += 1) {
      const marker = railMarkers[index]
      const previous = railMarkers[index - 1]
      const railTop = previous
        ? Math.min(Math.max(marker.railTop, previous.railTop + 30), maxRailTop)
        : marker.railTop
      marker.railTop = railTop
      marker.connectorY2 = railTop + 13
      marker.railOffset = railSide === 'left' ? -(index % 3) * 4 : (index % 3) * 4
    }
  }

  return markers
}

function moveAnnotationPoint(page: number, annotationId: string, local: PdfPoint) {
  const annotation = annotationsForPage(page).find(item => item.id === annotationId)
  if (annotation?.type !== 'note') return
  annotation.point = pdfPoint(local, page)
  store.dirty = true
  updateHistoryState()
  drawAnnotations(page)
}

function startMarkerDrag(event: PointerEvent, page: number, annotation: PdfAnnotation) {
  if (annotation.type !== 'note') return
  if (annotation.quoteRects?.length) return
  const target = event.currentTarget as HTMLElement
  target.setPointerCapture(event.pointerId)
  draggingMarker = { page, annotationId: annotation.id, pointerId: event.pointerId, moved: false }
  pushHistory()
}

function moveMarkerDrag(event: PointerEvent, page: number) {
  if (!draggingMarker || draggingMarker.page !== page || draggingMarker.pointerId !== event.pointerId) return
  const local = localPointerPosition(event, page)
  moveAnnotationPoint(page, draggingMarker.annotationId, local)
  draggingMarker.moved = true
}

function endMarkerDrag(event: PointerEvent, page: number, annotation: PdfAnnotation) {
  if (!draggingMarker || draggingMarker.page !== page || draggingMarker.pointerId !== event.pointerId) return
  const target = event.currentTarget as HTMLElement
  if (target.hasPointerCapture(event.pointerId)) target.releasePointerCapture(event.pointerId)
  const moved = draggingMarker.moved
  draggingMarker = null
  if (!moved) {
    openNoteEditor(page, annotation.type === 'note' ? annotation.point : { x: 0, y: 0 }, viewportPoint(annotation.type === 'note' ? annotation.point : { x: 0, y: 0 }, page), annotation)
  }
}

function openNoteEditor(page: number, point: PdfPoint, local: PdfPoint, annotation?: PdfAnnotation) {
  if (
    annotation?.type === 'note'
    && noteEditor.value.open
    && noteEditor.value.annotationId === annotation.id
  ) {
    noteEditor.value.open = false
    return
  }
  const viewport = pageViewport(page)
  noteEditor.value = {
    open: true,
    page,
    annotationId: annotation?.id ?? '',
    point,
    text: annotation?.type === 'note' ? annotation.text : '',
    quoteText: annotation?.type === 'note' ? annotation.quoteText ?? '' : '',
    quoteRects: annotation?.type === 'note' ? annotation.quoteRects ?? [] : [],
    translations: annotation?.type === 'note' ? annotation.translations ?? [] : [],
    left: Math.min(local.x + 16, Math.max((viewport?.width ?? 320) - 280, 8)),
    top: Math.min(local.y + 16, Math.max((viewport?.height ?? 220) - 190, 8)),
    dragging: false,
    dragOffsetX: 0,
    dragOffsetY: 0,
  }
  nextTick(() => document.querySelector<HTMLTextAreaElement>('.note-editor textarea')?.focus())
}

function openSelectedTextNoteEditor() {
  const page = textSelection.value.page
  if (!page || !pageViewport(page) || !textSelection.value.rects.length) return
  const rects = textSelection.value.rects.map(clientRect => {
    const rect = viewportRect(clientRect, page)
    return {
      start: pdfPoint({ x: rect.left, y: rect.top }, page),
      end: pdfPoint({ x: rect.right, y: rect.bottom }, page),
    }
  })
  const lastClientRect = textSelection.value.rects[textSelection.value.rects.length - 1]
  const lastRect = viewportRect(lastClientRect, page)
  const local = {
    x: Math.min(lastRect.right + 18, (pageViewport(page)?.width ?? lastRect.right) - 16),
    y: Math.max(lastRect.top + (lastRect.bottom - lastRect.top) / 2, 16),
  }
  const point = pdfPoint(local, page)
  noteEditor.value = {
    open: true,
    page,
    annotationId: '',
    point,
    text: '',
    quoteText: textSelection.value.text,
    quoteRects: rects,
    translations: [],
    left: Math.min(local.x + 16, Math.max((pageViewport(page)?.width ?? 320) - 280, 8)),
    top: Math.min(local.y + 16, Math.max((pageViewport(page)?.height ?? 220) - 190, 8)),
    dragging: false,
    dragOffsetX: 0,
    dragOffsetY: 0,
  }
  clearTextSelection(true)
  nextTick(() => document.querySelector<HTMLTextAreaElement>('.note-editor textarea')?.focus())
}

async function translateSelectedText() {
  if (!store.featureFlags.translation) return
  const page = textSelection.value.page
  const sourceText = textSelection.value.text.trim()
  const viewport = pageViewport(page)
  if (!page || !viewport || !sourceText) return

  const translator = normalizeTranslatorName(localStorage.getItem('funpdf.translator') || 'baidu')
  const targetLanguage = localStorage.getItem('funpdf.targetLanguage') || 'zh-CN'
  const sourceLanguage = localStorage.getItem('funpdf.sourceLanguage') || 'auto'
  const modelType = localStorage.getItem('funpdf.baidu.modelType') || 'nmt'
  const reference = localStorage.getItem('funpdf.baidu.reference') || ''
  const deeplRegion = localStorage.getItem('funpdf.deepl.region') || 'free'
  const deeplModelType = localStorage.getItem('funpdf.deepl.modelType') || 'prefer_quality_optimized'
  const deeplFormality = localStorage.getItem('funpdf.deepl.formality') || 'default'
  const deeplPreserveFormatting = localStorage.getItem('funpdf.deepl.preserveFormatting') === 'true'
  const googleFormat = localStorage.getItem('funpdf.google.format') || 'text'
  const azureTextType = localStorage.getItem('funpdf.azure.textType') || 'plain'
  const azureScript = localStorage.getItem('funpdf.azure.script') || ''
  const quoteRects = textSelection.value.rects.map(clientRect => {
    const rect = viewportRect(clientRect, page)
    return {
      start: pdfPoint({ x: rect.left, y: rect.top }, page),
      end: pdfPoint({ x: rect.right, y: rect.bottom }, page),
    }
  })
  const lastClientRect = textSelection.value.rects[textSelection.value.rects.length - 1]
  const lastRect = viewportRect(lastClientRect, page)
  const popupLeft = Math.min(Math.max(lastRect.right + 18, 8), Math.max(viewport.width - 286, 8))
  const popupWidth = Math.max(Math.min(320, viewport.width - popupLeft - 8), 220)
  const markerLocal = {
    x: Math.min(Math.max(lastRect.right + 26, 22), viewport.width - 28),
    y: Math.min(Math.max(lastRect.bottom + 5, 18), viewport.height - 18),
  }
  translationPopup.value = {
    open: true,
    page,
    sourceText,
    result: '',
    error: '',
    loading: true,
    left: popupLeft,
    top: Math.min(Math.max(lastRect.top, 8), Math.max(viewport.height - 240, 8)),
    width: popupWidth,
    point: pdfPoint(markerLocal, page),
    quoteText: sourceText,
    quoteRects,
    translator,
    targetLanguage,
  }

  try {
    const response = await completeTranslation(translator, {
      text: sourceText,
      source_language: sourceLanguage === 'auto' ? undefined : sourceLanguage,
      target_language: targetLanguage,
      region: translator === 'deepl' ? deeplRegion : undefined,
      params: translator === 'baidu'
        ? { model_type: modelType, reference: reference.trim() || undefined }
        : translator === 'deepl'
          ? { model_type: deeplModelType, formality: deeplFormality, preserve_formatting: deeplPreserveFormatting }
          : translator === 'google'
            ? { format: googleFormat }
            : translator === 'azure'
              ? { textType: azureTextType, script: azureScript.trim() || undefined }
              : {},
    })
    translationPopup.value.result = response.translated_text
  } catch (requestError) {
    translationPopup.value.error = apiErrorMessage(requestError, '翻译失败')
  } finally {
    translationPopup.value.loading = false
  }
}

function askAIAboutSelection() {
  if (!store.featureFlags.aiChat) return
  const sourceText = textSelection.value.text.trim()
  if (!sourceText) return
  store.openAIChat(sourceText)
  clearTextSelection(true)
}

async function getDocumentContext() {
  const document = pdfDocument.value
  if (!document || !store.documentName) return undefined
  const pages: string[] = []
  for (let pageNumber = 1; pageNumber <= document.numPages; pageNumber++) {
    const page = await document.getPage(pageNumber)
    const content = await page.getTextContent()
    const text = content.items.map(item => 'str' in item ? item.str : '').join(' ')
    pages.push(`Page ${pageNumber}\n${text}`)
  }
  return { name: store.documentName, content: pages.join('\n\n') }
}

function saveTranslationResult() {
  const page = translationPopup.value.page
  const translatedText = translationPopup.value.result.trim()
  if (!page || !translatedText) return
  const translation: NoteTranslation = {
    id: makeId(),
    sourceText: translationPopup.value.quoteText,
    translatedText,
    translator: translationPopup.value.translator,
    targetLanguage: translationPopup.value.targetLanguage,
    createdAt: new Date().toISOString(),
  }
  pushHistory()
  annotationsForPage(page).push({
    id: makeId(),
    page,
    type: 'note',
    color: store.annotationColor,
    width: 1,
    point: translationPopup.value.point,
    text: '',
    quoteText: translation.sourceText,
    quoteRects: translationPopup.value.quoteRects,
    translations: [translation],
  })
  translationPopup.value.open = false
  clearTextSelection(true)
  updateHistoryState()
  drawAnnotations(page)
  setStatus('翻译已保存')
}

function saveNote() {
  const text = noteEditor.value.text.trim()
  if (!text && noteEditor.value.translations.length === 0) { noteEditor.value.open = false; return }
  const page = noteEditor.value.page
  pushHistory()
  const existing = annotationsForPage(page).find(annotation => annotation.id === noteEditor.value.annotationId)
  if (existing?.type === 'note') {
    existing.text = text
    existing.point = noteEditor.value.point
    existing.quoteText = noteEditor.value.quoteText || undefined
    existing.quoteRects = noteEditor.value.quoteRects.length ? noteEditor.value.quoteRects : undefined
    existing.translations = noteEditor.value.translations.length ? noteEditor.value.translations : undefined
  } else {
    annotationsForPage(page).push({
      id: makeId(),
      page,
      type: 'note',
      color: store.annotationColor,
      width: 1,
      point: noteEditor.value.point,
      text,
      quoteText: noteEditor.value.quoteText || undefined,
      quoteRects: noteEditor.value.quoteRects.length ? noteEditor.value.quoteRects : undefined,
      translations: noteEditor.value.translations.length ? noteEditor.value.translations : undefined,
    })
  }
  noteEditor.value.open = false
  updateHistoryState()
  drawAnnotations(page)
}

function startNoteEditorDrag(event: PointerEvent) {
  const viewport = pageViewport(noteEditor.value.page)
  if (!viewport) return
  const target = event.currentTarget as HTMLElement
  target.setPointerCapture(event.pointerId)
  noteEditor.value.dragging = true
  noteEditor.value.dragOffsetX = event.offsetX
  noteEditor.value.dragOffsetY = event.offsetY
}

function moveNoteEditorDrag(event: PointerEvent) {
  if (!noteEditor.value.dragging) return
  const viewport = pageViewport(noteEditor.value.page)
  if (!viewport) return
  const pageElement = pageElements.get(noteEditor.value.page)
  const pageRect = pageElement?.getBoundingClientRect()
  if (!pageRect) return
  const left = Math.min(Math.max(event.clientX - pageRect.left - noteEditor.value.dragOffsetX, 8), Math.max(viewport.width - 280, 8))
  const top = Math.min(Math.max(event.clientY - pageRect.top - noteEditor.value.dragOffsetY, 8), Math.max(viewport.height - 190, 8))
  noteEditor.value.left = left
  noteEditor.value.top = top
  noteEditor.value.point = pdfPoint({ x: left, y: top }, noteEditor.value.page)
}

function endNoteEditorDrag(event: PointerEvent) {
  if (!noteEditor.value.dragging) return
  noteEditor.value.dragging = false
  const target = event.currentTarget as HTMLElement
  if (target.hasPointerCapture(event.pointerId)) target.releasePointerCapture(event.pointerId)
}

function clearTextSelection(removeNative = false) {
  textSelection.value = { open: false, page: 0, text: '', rects: [], left: 0, top: 0 }
  store.selectedText = ''
  if (removeNative) window.getSelection()?.removeAllRanges()
}

function handleSelectionChange() {
  const selection = window.getSelection()
  if (!selection || selection.isCollapsed || selection.rangeCount === 0) return clearTextSelection()
  let selectedPage = 0
  let container: HTMLDivElement | undefined
  for (const [page, layer] of textLayerElements) {
    if (selection.anchorNode && selection.focusNode && layer.contains(selection.anchorNode) && layer.contains(selection.focusNode)) {
      selectedPage = page
      container = layer
      break
    }
  }
  if (!container || !selectedPage) return clearTextSelection()
  const text = selection.toString().trim()
  const containerRect = container.getBoundingClientRect()
  const rects = Array.from(selection.getRangeAt(0).getClientRects()).filter(rect => rect.width > 0 && rect.height > 0)
  if (!text || !rects.length) return clearTextSelection()
  const first = rects[0]
  textSelection.value = {
    open: true,
    page: selectedPage,
    text,
    rects,
    left: Math.min(Math.max(first.left + first.width / 2 - containerRect.left, 112), Math.max(containerRect.width - 112, 112)),
    top: Math.max(first.top - containerRect.top - 46, 8),
  }
  store.selectedText = text
  setCurrentPageFromView(selectedPage)
}

function viewportRect(rect: DOMRect, page: number) {
  const containerRect = textLayerElements.get(page)!.getBoundingClientRect()
  const viewport = pageViewport(page)!
  const sx = viewport.width / containerRect.width
  const sy = viewport.height / containerRect.height
  return {
    left: (Math.max(rect.left, containerRect.left) - containerRect.left) * sx,
    top: (Math.max(rect.top, containerRect.top) - containerRect.top) * sy,
    right: (Math.min(rect.right, containerRect.right) - containerRect.left) * sx,
    bottom: (Math.min(rect.bottom, containerRect.bottom) - containerRect.top) * sy,
  }
}

function highlightSelectedText() {
  const page = textSelection.value.page
  if (!page || !pageViewport(page) || !textSelection.value.rects.length) return
  pushHistory()
  for (const clientRect of textSelection.value.rects) {
    const rect = viewportRect(clientRect, page)
    annotationsForPage(page).push({
      id: makeId(), page, type: 'highlight', color: store.annotationColor, width: 1,
      start: pdfPoint({ x: rect.left, y: rect.top }, page),
      end: pdfPoint({ x: rect.right, y: rect.bottom }, page),
    })
  }
  updateHistoryState()
  clearTextSelection(true)
  drawAnnotations(page)
  setStatus('已高亮选中文字')
}

async function copySelectedText() {
  if (!textSelection.value.text) return
  try { await navigator.clipboard.writeText(textSelection.value.text); setStatus('选中文字已复制') }
  catch { setStatus('复制失败，请使用 Ctrl+C') }
}

function undo() {
  const previous = undoStack.pop()
  if (!previous) return
  redoStack.push(cloneAnnotations())
  annotations = cloneAnnotations(previous)
  store.dirty = true
  updateHistoryState()
  redrawAnnotations()
}

function redo() {
  const next = redoStack.pop()
  if (!next) return
  undoStack.push(cloneAnnotations())
  annotations = cloneAnnotations(next)
  store.dirty = true
  updateHistoryState()
  redrawAnnotations()
}

function clearAnnotations() {
  if (!store.annotationCount) return
  pushHistory()
  annotations = {}
  updateHistoryState()
  redrawAnnotations()
  setStatus('已清除全部标注，可使用撤销恢复')
}

async function fitWidth() {
  await setInitialFitWidth()
}

async function rotate() {
  rotation.value = (rotation.value + 90) % 360
  store.pageThumbnails = {}
  await refreshPageFlow(true)
  void generateThumbnails()
}

function handleStageScroll() {
  if (scrollFrame) return
  scrollFrame = window.requestAnimationFrame(() => {
    scrollFrame = 0
    const stage = stageRef.value
    if (!stage || !pageElements.size) return
    const stageRect = stage.getBoundingClientRect()
    const readingLine = stageRect.top + Math.min(stageRect.height * 0.34, 260)
    let closestPage = store.currentPage
    let closestDistance = Number.POSITIVE_INFINITY
    for (const [page, element] of pageElements) {
      const rect = element.getBoundingClientRect()
      const distance = readingLine >= rect.top && readingLine <= rect.bottom
        ? 0
        : Math.min(Math.abs(readingLine - rect.top), Math.abs(readingLine - rect.bottom))
      if (distance < closestDistance) { closestDistance = distance; closestPage = page }
    }
    setCurrentPageFromView(closestPage)
    renderVisiblePages()
  })
}

function handleStageWheel(event: WheelEvent) {
  if (!(event.ctrlKey || event.metaKey)) return
  event.preventDefault()
  event.stopPropagation()
  if (!store.totalPages) return
  if (event.deltaY < 0) store.zoomIn()
  else if (event.deltaY > 0) store.zoomOut()
}

function scrollToPage(page: number, smooth = true) {
  const stage = stageRef.value
  const element = pageElements.get(page)
  if (!stage || !element) return
  stage.scrollTo({ top: Math.max(element.offsetTop - 22, 0), behavior: smooth ? 'smooth' : 'auto' })
  void renderPage(page)
}

function annotationViewportPoint(annotation: PdfAnnotation, page: number) {
  if (annotation.type !== 'note') return undefined
  const viewport = pageViewport(page)
  if (!viewport) return undefined
  const quoteRects = annotation.quoteRects ?? []
  if (!quoteRects.length) return viewportPoint(annotation.point, page)
  const firstQuoteRect = quoteRects[0]
  const lastQuoteRect = quoteRects[quoteRects.length - 1]
  const start = viewportPoint(firstQuoteRect.start, page)
  const end = viewportPoint(lastQuoteRect.end, page)
  return {
    x: Math.min(Math.max(end.x + 4, 8), Math.max(viewport.width - 8, 8)),
    y: Math.min(Math.max((start.y + end.y) / 2, 10), Math.max(viewport.height - 10, 10)),
  }
}

async function focusAnnotation(annotationId: string, page: number) {
  const stage = stageRef.value
  const pageElement = pageElements.get(page)
  const annotation = annotationsForPage(page).find(item => item.id === annotationId)
  if (!stage || !pageElement || !annotation) {
    scrollToPage(page)
    return
  }

  setCurrentPageFromView(page)
  await renderPage(page)
  await nextTick()
  const point = annotationViewportPoint(annotation, page)
  const top = point
    ? pageElement.offsetTop + point.y - stage.clientHeight * 0.38
    : pageElement.offsetTop - 22
  stage.scrollTo({ top: Math.max(top, 0), behavior: 'smooth' })
  focusedAnnotationId.value = annotationId
  window.setTimeout(() => {
    if (focusedAnnotationId.value === annotationId) focusedAnnotationId.value = ''
  }, 1800)
}

function hexToRgb(color: string) {
  const value = color.replace('#', '')
  const normalized = value.length === 3 ? value.split('').map(char => char + char).join('') : value
  return rgb(Number.parseInt(normalized.slice(0, 2), 16) / 255, Number.parseInt(normalized.slice(2, 4), 16) / 255, Number.parseInt(normalized.slice(4, 6), 16) / 255)
}

async function createFlattenedNoteImage(output: PDFDocument, text: string, color: string) {
  const canvas = document.createElement('canvas')
  canvas.width = 720
  canvas.height = 400
  const context = canvas.getContext('2d')
  if (!context) throw new Error('Canvas is unavailable')
  context.fillStyle = '#fff8cf'
  context.fillRect(0, 0, canvas.width, canvas.height)
  context.strokeStyle = color
  context.lineWidth = 10
  context.strokeRect(5, 5, canvas.width - 10, canvas.height - 10)
  context.fillStyle = '#4b4332'
  context.font = 'bold 34px "Microsoft YaHei", sans-serif'
  context.fillText('便签', 36, 58)
  context.font = '28px "Microsoft YaHei", sans-serif'
  const maxWidth = canvas.width - 72
  const lineHeight = 42
  let line = ''
  let y = 112
  for (const character of text) {
    const next = line + character
    if (context.measureText(next).width > maxWidth || character === '\n') {
      context.fillText(line, 36, y)
      line = character === '\n' ? '' : character
      y += lineHeight
      if (y > canvas.height - 40) break
    } else {
      line = next
    }
  }
  if (line && y <= canvas.height - 40) context.fillText(line, 36, y)
  const blob = await new Promise<Blob>((resolve, reject) => canvas.toBlob(value => value ? resolve(value) : reject(new Error('Could not render note')), 'image/png'))
  return output.embedPng(new Uint8Array(await blob.arrayBuffer()))
}

async function buildFlattenedPdf() {
  if (!originalBytes) throw new Error('No PDF loaded')
  const output = await PDFDocument.load(originalBytes.slice())
  const pages = output.getPages()
  for (const pageAnnotations of Object.values(annotations)) {
    for (const annotation of pageAnnotations) {
      const page = pages[annotation.page - 1]
      if (!page) continue
      const color = hexToRgb(annotation.color)
      if (annotation.type === 'pen') {
        for (let index = 1; index < annotation.points.length; index++) page.drawLine({ start: annotation.points[index - 1], end: annotation.points[index], thickness: Math.max(annotation.width, 0.5), color, opacity: 0.95 })
      } else if (annotation.type === 'highlight') {
        page.drawRectangle({ x: Math.min(annotation.start.x, annotation.end.x), y: Math.min(annotation.start.y, annotation.end.y), width: Math.abs(annotation.end.x - annotation.start.x), height: Math.abs(annotation.end.y - annotation.start.y), color, opacity: 0.28, borderWidth: 0 })
      } else if (annotation.type === 'underline' || annotation.type === 'strike') {
        page.drawLine({ start: annotation.start, end: annotation.end, thickness: Math.max(annotation.width, 0.5), color, opacity: 0.95 })
      } else if (annotation.type === 'note') {
        const flattenedText = annotation.text || annotation.translations?.map(item => item.translatedText).join('\n\n') || ''
        const image = await createFlattenedNoteImage(output, flattenedText, annotation.color)
        const size = page.getSize()
        const width = Math.min(180, size.width)
        const height = width * image.height / image.width
        page.drawImage(image, {
          x: Math.min(Math.max(annotation.point.x, 0), Math.max(size.width - width, 0)),
          y: Math.min(Math.max(annotation.point.y - height, 0), Math.max(size.height - height, 0)),
          width,
          height,
          opacity: 0.96,
        })
      }
    }
  }
  return output.save()
}

function makePdfBlob(bytes: Uint8Array) {
  return new Blob([bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) as ArrayBuffer], { type: 'application/pdf' })
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

async function saveProject(options: { quiet?: boolean; thumbnail?: string } = {}) {
  if (!originalBytes || saving.value) return false
  saving.value = true
  try {
    const savedAt = new Date().toISOString()
    const editor: FunPdfEditorState['editor'] = {
      annotations: cloneAnnotations(),
      rotation: rotation.value,
      scale: store.scale,
      current_page: store.currentPage,
    }

    const state: FunPdfEditorState = {
      format: FUNPDF_EDITOR_STATE_FORMAT,
      version: FUNPDF_PROJECT_VERSION,
      saved_at: savedAt,
      source: {
        name: store.documentName,
        mime_type: 'application/pdf',
        sha256: cachedFileSha256 || undefined,
      },
      editor,
    }

    if (cachedFileId) {
      const result = await saveEditorState(
        cachedFileId,
        cachedFileRevision,
        state,
        options.thumbnail,
      )
      cachedFileRevision = result.revision
      store.dirty = false
      snapshotActiveTab()
      return true
    }

    const sourceFile = new File([makePdfBlob(originalBytes)], store.documentName || 'document.pdf', {
      type: 'application/pdf',
    })
    const cached = await cachePdfFile(sourceFile, state)
    cachedFileId = cached.id
    cachedFileRevision = cached.revision
    cachedFileSha256 = cached.sha256
    store.activeCachedFileId = cachedFileId
    store.dirty = false
    snapshotActiveTab()
    const tab = activeTab()
    if (tab && !tab.autosaveTimer) startAutosave(tab)
    window.dispatchEvent(new Event('funpdf:files-changed'))
    return true
  } catch (error: any) {
    if (error?.name !== 'AbortError') {
      console.error(error)
      setStatus(apiErrorMessage(error, '保存工程失败，请重试'))
    }
    return false
  } finally {
    saving.value = false
  }
}

async function exportPdf() {
  if (!originalBytes || exporting.value || saving.value) return
  if (!await saveProject({ quiet: true })) return
  exporting.value = true
  try {
    const filename = `${store.documentName.replace(/\.pdf$/i, '') || 'document'}-扁平化.pdf`
    downloadBlob(makePdfBlob(await buildFlattenedPdf()), filename)
    setStatus('工程已保存，并已导出扁平化 PDF')
  } catch (error) { console.error(error); setStatus('导出失败，请重试') }
  finally { exporting.value = false }
}

async function printPdf() {
  if (!originalBytes || exporting.value) return
  exporting.value = true
  try {
    const url = URL.createObjectURL(makePdfBlob(await buildFlattenedPdf()))
    const frame = document.createElement('iframe')
    Object.assign(frame.style, { position: 'fixed', width: '1px', height: '1px', opacity: '0' })
    frame.src = url
    frame.onload = () => frame.contentWindow?.print()
    document.body.appendChild(frame)
    window.setTimeout(() => { frame.remove(); URL.revokeObjectURL(url) }, 60_000)
  } catch (error) { console.error(error); setStatus('无法准备打印文件，请重试') }
  finally { exporting.value = false }
}

function handleKeydown(event: KeyboardEvent) {
  if (!(event.ctrlKey || event.metaKey)) return
  const key = event.key.toLowerCase()
  if (key === 'z') { event.preventDefault(); event.shiftKey ? redo() : undo() }
  else if (key === 'y') { event.preventDefault(); redo() }
  else if (key === 's' && store.totalPages) { event.preventDefault(); void saveProject() }
}

function handleBeforeUnload(event: BeforeUnloadEvent) {
  snapshotActiveTab()
  if (!store.dirty && !openTabs.value.some(tab => tab.dirty)) return
  event.preventDefault()
  event.returnValue = ''
}

function handleFocusAnnotation(event: Event) {
  const detail = (event as CustomEvent<{ annotationId?: string; page?: number }>).detail
  const annotationId = detail?.annotationId
  const page = detail?.page
  if (!annotationId || !page) return
  void focusAnnotation(annotationId, page)
}

defineExpose({ openFileDialog, closeDocument, rotate, fitWidth, undo, redo, clearAnnotations, saveProject, exportPdf, printPdf, getDocumentContext })

watch(() => store.scale, () => void refreshPageFlow(true))
watch(() => store.dirty, dirty => {
  const tab = activeTab()
  if (tab) tab.dirty = dirty
})
watch(() => store.currentPage, page => {
  if (pageChangeCameFromScroll) { pageChangeCameFromScroll = false; return }
  const normalized = Math.min(Math.max(Math.round(page || 1), 1), Math.max(store.totalPages, 1))
  if (normalized !== page) { store.currentPage = normalized; return }
  scrollToPage(normalized)
}, { flush: 'sync' })
watch(() => store.activeTool, (tool, previous) => {
  if (tool === 'highlight' && previous === 'cursor' && textSelection.value.open) return highlightSelectedText()
  if (store.featureFlags.notes && tool === 'note' && previous === 'cursor' && textSelection.value.open) return openSelectedTextNoteEditor()
  if (tool !== 'cursor') clearTextSelection(true)
})
watch(() => store.featureFlags.notes, enabled => {
  if (!enabled) {
    noteEditor.value.open = false
    if (store.activeTool === 'note') store.activeTool = 'cursor'
  }
})
watch(() => store.featureFlags.translation, enabled => {
  if (!enabled) translationPopup.value.open = false
})

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('beforeunload', handleBeforeUnload)
  document.addEventListener('selectionchange', handleSelectionChange)
  window.addEventListener('funpdf:open-cached-file', handleOpenCachedFile)
  window.addEventListener('funpdf:focus-annotation', handleFocusAnnotation)
  stopDesktopFileDrop = onDesktopFileDrop(paths => {
    void handleDesktopFileDrop(paths)
  })
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('beforeunload', handleBeforeUnload)
  document.removeEventListener('selectionchange', handleSelectionChange)
  window.removeEventListener('funpdf:open-cached-file', handleOpenCachedFile)
  window.removeEventListener('funpdf:focus-annotation', handleFocusAnnotation)
  stopDesktopFileDrop?.()
  openTabs.value.forEach(stopAutosave)
  pageObserver?.disconnect()
  if (scrollFrame) cancelAnimationFrame(scrollFrame)
  cancelPageRendering()
  void pdfLoadingTask?.destroy()
})
</script>

<template>
  <section class="viewer">
    <nav v-if="openTabs.length" class="document-tabs" aria-label="打开的 PDF">
      <button
        v-for="tab in openTabs"
        :key="tab.id"
        class="document-tab"
        :class="{ active: tab.id === activeTabId }"
        :title="tab.name"
        @click="activateTab(tab.id)"
      >
        <i class="fa-regular fa-file-pdf"></i>
        <span>{{ tab.id === activeTabId && store.dirty ? '● ' : tab.dirty ? '● ' : '' }}{{ tab.name }}</span>
        <em v-if="tab.saving || (tab.id === activeTabId && saving)">
          <i class="fa-solid fa-circle-notch fa-spin"></i>
        </em>
        <i
          v-else
          class="fa-solid fa-xmark close-tab"
          title="关闭"
          @click.stop="closeTab(tab.id)"
        ></i>
      </button>
    </nav>
    <input ref="fileInputRef" class="hidden-input" type="file" accept="application/pdf,.pdf,.funpdf" @change="handleFileChange" />
    <div
      ref="stageRef"
      class="page-stage"
      @wheel="handleStageWheel"
      @scroll.passive="handleStageScroll"
      @dragenter.prevent="dragActive = true"
      @dragover="handleDragOver"
      @dragleave="handleDragLeave"
      @drop="handleDrop"
    >
      <div v-if="store.totalPages === 0" class="empty-state">
        <div class="empty-icon"><i class="fa-regular fa-file-pdf"></i></div>
        <h1>打开一个 PDF 开始编辑</h1>
        <p>文件只在你的浏览器中处理。打开后可使用上方工具进行画笔、高亮、划线、擦除与便签标注。</p>
        <button @click="openFileDialog"><i class="fa-regular fa-folder-open"></i>选择本地 PDF</button>
        <span class="drop-hint">也可以将文件拖放到这里</span>
      </div>

      <div v-else class="page-flow">
        <article
          v-for="layout in pageLayouts"
          :key="layout.pageNumber"
          :ref="element => setPageElement(element, layout.pageNumber)"
          class="page-shell"
          :class="{ current: store.currentPage === layout.pageNumber }"
          :data-page="layout.pageNumber"
          :data-active-tool="store.activeTool"
          :style="{ width: `${layout.width}px`, height: `${layout.height}px` }"
        >
          <canvas :ref="element => setPageCanvas(element, layout.pageNumber)" class="pdf-canvas"></canvas>
          <div :ref="element => setTextLayer(element, layout.pageNumber)" class="textLayer text-layer"></div>
          <canvas
            :ref="element => setAnnotationCanvas(element, layout.pageNumber)"
            class="annotation-canvas"
            :data-tool="store.activeTool"
            @pointerdown="startPointer($event, layout.pageNumber)"
            @pointermove="movePointer($event, layout.pageNumber)"
            @pointerup="endPointer($event, layout.pageNumber)"
            @pointercancel="endPointer($event, layout.pageNumber)"
          ></canvas>

          <div class="link-layer" aria-label="PDF 链接">
            <template v-for="link in pageLinks[layout.pageNumber] ?? []" :key="link.id">
              <a
                v-if="link.url"
                class="pdf-link"
                :href="link.url"
                :title="link.title"
                :style="{ left: `${link.left}px`, top: `${link.top}px`, width: `${link.width}px`, height: `${link.height}px` }"
                target="_blank"
                rel="noopener noreferrer"
                @click="setCurrentPageFromView(layout.pageNumber)"
              ></a>
              <button
                v-else
                class="pdf-link"
                :title="link.title"
                :style="{ left: `${link.left}px`, top: `${link.top}px`, width: `${link.width}px`, height: `${link.height}px` }"
                @click="activatePdfLink(link, $event)"
              ></button>
            </template>
          </div>

          <template v-for="marker in noteMarkers(layout.pageNumber)" :key="marker.annotation.id">
            <svg
              v-if="marker.annotation.quoteRects?.length"
              class="note-connector"
              :class="{ 'translation-connector': marker.isTranslation, active: focusedAnnotationId === marker.annotation.id }"
              :style="{ left: '0px', top: '0px', width: `${layout.width}px`, height: `${layout.height}px` }"
              aria-hidden="true"
            >
              <line
                :x1="marker.connectorX1"
                :y1="marker.connectorY1"
                :x2="marker.connectorX2"
                :y2="marker.connectorY2"
              />
            </svg>
            <span
              v-if="marker.annotation.quoteRects?.length"
              class="text-anchor"
              :class="{ 'translation-anchor': marker.isTranslation, active: focusedAnnotationId === marker.annotation.id }"
              :style="{ left: `${marker.point.x}px`, top: `${marker.point.y}px` }"
              aria-hidden="true"
            ></span>
            <button
              class="note-marker"
              :class="{ 'translation-marker': marker.isTranslation, 'rail-marker': marker.annotation.quoteRects?.length, focused: focusedAnnotationId === marker.annotation.id }"
              :style="marker.annotation.quoteRects?.length ? { left: `${marker.railLeft + marker.railOffset}px`, top: `${marker.railTop}px`, backgroundColor: marker.isTranslation ? '#f4f8fb' : '#fff8db', borderColor: marker.isTranslation ? '#9fc4dc' : marker.annotation.color, color: marker.isTranslation ? '#315b72' : '#7a5416' } : { left: `${marker.point.x}px`, top: `${marker.point.y}px`, backgroundColor: marker.isTranslation ? '#eeeeee' : marker.annotation.color, borderColor: marker.isTranslation ? '#c9c9c9' : marker.annotation.color, color: marker.isTranslation ? '#4f555b' : '#ffffff' }"
              :title="marker.isTranslation ? 'Open saved translation' : 'Open note'"
              @pointerdown.stop.prevent="startMarkerDrag($event, layout.pageNumber, marker.annotation)"
              @pointermove.stop.prevent="moveMarkerDrag($event, layout.pageNumber)"
              @pointerup.stop.prevent="endMarkerDrag($event, layout.pageNumber, marker.annotation)"
              @pointercancel.stop.prevent="endMarkerDrag($event, layout.pageNumber, marker.annotation)"
              @click.stop.prevent="marker.annotation.quoteRects?.length && openNoteEditor(layout.pageNumber, marker.annotation.point, marker.point, marker.annotation)"
            >{{ marker.label }}</button>
          </template>

          <div
            v-if="textSelection.open && textSelection.page === layout.pageNumber"
            class="selection-toolbar"
            :style="{ left: `${textSelection.left}px`, top: `${textSelection.top}px` }"
            @mousedown.prevent
          >
            <button @click="copySelectedText"><i class="fa-regular fa-copy"></i>复制</button>
            <span></span>
            <button @click="highlightSelectedText"><i class="fa-solid fa-highlighter"></i>高亮</button>
            <template v-if="store.featureFlags.notes || store.featureFlags.translation">
              <span></span>
              <button v-if="store.featureFlags.notes" @click="openSelectedTextNoteEditor"><i class="fa-regular fa-note-sticky"></i>便签</button>
              <button v-if="store.featureFlags.translation" @click="translateSelectedText"><i class="fa-solid fa-language"></i>翻译</button>
            </template>
            <button v-if="store.featureFlags.aiChat" @click="askAIAboutSelection"><i class="fa-regular fa-comments"></i>问 AI</button>
          </div>

          <div
            v-if="store.featureFlags.translation && translationPopup.open && translationPopup.page === layout.pageNumber"
            class="translation-popup"
            :style="{ left: `${translationPopup.left}px`, top: `${translationPopup.top}px`, width: `${translationPopup.width}px` }"
            @pointerdown.stop
          >
            <div class="translation-popup-header">
              <strong>翻译</strong>
              <button title="关闭" @click="translationPopup.open = false">
                <i class="fa-solid fa-xmark"></i>
              </button>
            </div>
            <p v-if="translationPopup.loading" class="translation-loading">
              <i class="fa-solid fa-circle-notch fa-spin"></i> 翻译中…
            </p>
            <p v-else-if="translationPopup.error" class="translation-error">{{ translationPopup.error }}</p>
            <template v-else>
              <p v-if="translationPopup.result" class="translation-result">{{ translationPopup.result }}</p>
              <button v-if="translationPopup.result" class="save-translation-button" @click="saveTranslationResult">保存</button>
            </template>
          </div>

          <div
            v-if="store.featureFlags.notes && noteEditor.open && noteEditor.page === layout.pageNumber"
            class="note-editor"
            :style="{ left: `${noteEditor.left}px`, top: `${noteEditor.top}px` }"
            @pointermove="moveNoteEditorDrag"
            @pointerup="endNoteEditorDrag"
            @pointercancel="endNoteEditorDrag"
            @pointerdown.stop
          >
            <div class="note-editor-header" @pointerdown.stop.prevent="startNoteEditorDrag">
              <strong>{{ noteEditor.annotationId ? '编辑便签' : '添加便签' }}</strong>
              <i class="fa-solid fa-grip-lines"></i>
            </div>
            <blockquote v-if="noteEditor.quoteText">{{ noteEditor.quoteText }}</blockquote>
            <section class="note-group">
              <span>批注</span>
              <textarea v-model="noteEditor.text" maxlength="500" placeholder="输入备注内容…"></textarea>
            </section>
            <section v-if="noteEditor.translations.length" class="note-group">
              <span>翻译</span>
              <article v-for="item in noteEditor.translations" :key="item.id" class="saved-translation">
                <small>{{ item.translator }} · {{ item.targetLanguage }}</small>
                <p>{{ item.translatedText }}</p>
              </article>
            </section>
            <div class="note-editor-actions"><button class="secondary" @click="noteEditor.open = false">取消</button><button @click="saveNote">保存</button></div>
          </div>
        </article>
      </div>

      <div v-if="dragActive" class="drop-overlay"><i class="fa-regular fa-file-pdf"></i><strong>松开以打开 PDF</strong></div>
      <div v-if="(loading || saving || exporting) && store.totalPages > 0" class="loading"><i class="fa-solid fa-circle-notch fa-spin"></i>{{ exporting ? '正在生成扁平化 PDF…' : saving ? '正在保存工程…' : '正在准备页面…' }}</div>
    </div>

    <div v-if="store.statusMessage" class="status-message">{{ store.statusMessage }}</div>
  </section>
</template>

<style scoped>
.viewer { min-width: 0; min-height: 0; height: 100%; display: flex; flex-direction: column; position: relative; background: #e9e9e9; }
.document-tabs { height: 36px; flex: 0 0 36px; display: flex; align-items: flex-end; gap: 2px; padding: 5px 8px 0; overflow-x: auto; overflow-y: hidden; background: #f2f2f2; border-bottom: 1px solid #d9d9d9; }
.document-tab { max-width: 220px; min-width: 108px; height: 31px; padding: 0 8px; display: flex; align-items: center; gap: 7px; border: 1px solid transparent; border-bottom: 0; border-radius: 8px 8px 0 0; background: transparent; color: #5d6369; cursor: pointer; font-size: 12px; }
.document-tab:hover { background: #e8e8e8; }
.document-tab.active { background: #e9e9e9; color: #2f3439; border-color: #d9d9d9; }
.document-tab span { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; text-align: left; }
.document-tab em, .close-tab { width: 18px; height: 18px; flex: 0 0 18px; display: grid; place-items: center; border-radius: 5px; font-style: normal; font-size: 11px; }
.close-tab:hover { background: #d9d9d9; color: #22272c; }
.hidden-input { display: none; }
.page-stage { flex: 1; min-height: 0; overflow: auto; position: relative; padding: 28px 118px 40px; scroll-behavior: auto; }
.page-flow { width: max-content; min-width: 100%; display: flex; flex-direction: column; align-items: center; gap: 16px; }
.page-shell { position: relative; flex: 0 0 auto; overflow: visible; background: white; box-shadow: 0 2px 9px rgb(0 0 0 / 12%), 0 12px 26px rgb(0 0 0 / 8%); }
.page-shell.current { outline: 1px solid rgb(85 90 96 / 22%); }
.pdf-canvas { display: block; background: white; }
.text-layer { position: absolute; inset: 0; z-index: 1; overflow: clip; line-height: 1; letter-spacing: normal; word-spacing: normal; text-align: initial; transform-origin: 0 0; -webkit-text-size-adjust: none; text-size-adjust: none; pointer-events: none; user-select: none; --min-font-size: 1; --text-scale-factor: calc(var(--total-scale-factor) * var(--min-font-size)); --min-font-size-inv: calc(1 / var(--min-font-size)); }
.page-shell[data-active-tool='cursor'] .text-layer { pointer-events: auto; user-select: text; }
.text-layer :deep(span), .text-layer :deep(br) { position: absolute; color: transparent; white-space: pre; cursor: text; transform-origin: 0 0; user-select: text; }
.text-layer :deep(> :not(.markedContent)), .text-layer :deep(.markedContent span:not(.markedContent)) { z-index: 1; --font-height: 0; --scale-x: 1; --rotate: 0deg; font-size: calc(var(--text-scale-factor) * var(--font-height)); transform: rotate(var(--rotate)) scaleX(var(--scale-x)) scale(var(--min-font-size-inv)); }
.text-layer :deep(.markedContent) { display: contents; }
.text-layer :deep(::selection) { color: transparent; background: rgb(90 104 120 / 25%); }
.annotation-canvas { position: absolute; inset: 0; z-index: 2; display: block; touch-action: none; }
.annotation-canvas[data-tool='cursor'] { pointer-events: none; cursor: text; }
.annotation-canvas[data-tool='pen'], .annotation-canvas[data-tool='highlight'], .annotation-canvas[data-tool='underline'], .annotation-canvas[data-tool='strike'] { cursor: crosshair; }
.annotation-canvas[data-tool='eraser'] { cursor: cell; }
.annotation-canvas[data-tool='note'] { cursor: copy; }
.link-layer { position: absolute; inset: 0; z-index: 3; pointer-events: none; }
.pdf-link { position: absolute; display: block; padding: 0; border: 0; border-radius: 2px; background: transparent; pointer-events: none; }
.page-shell[data-active-tool='cursor'] .pdf-link { pointer-events: auto; cursor: pointer; }
.page-shell[data-active-tool='cursor'] .pdf-link:hover { outline: 1px solid rgb(75 85 99 / 35%); background: rgb(120 130 140 / 8%); }
.pdf-link:focus-visible { outline: 2px solid #555b62; outline-offset: 1px; }
.note-connector { position: absolute; z-index: 3; overflow: visible; pointer-events: none; }
.note-connector line { stroke: #b7791f; stroke-width: 1.8; stroke-dasharray: 3 3; vector-effect: non-scaling-stroke; }
.note-connector.translation-connector line { stroke: #2563eb; }
.note-connector.active line { stroke-width: 2.6; stroke-dasharray: none; }
.text-anchor { position: absolute; z-index: 4; width: 10px; height: 10px; transform: translate(-50%, -50%); border: 2px solid #b7791f; border-radius: 50%; background: #f59e0b; box-shadow: 0 0 0 4px rgb(245 158 11 / 22%); pointer-events: none; }
.text-anchor.translation-anchor { border-color: #1d4ed8; background: #3b82f6; box-shadow: 0 0 0 4px rgb(37 99 235 / 22%); }
.text-anchor.active { box-shadow: 0 0 0 6px rgb(245 158 11 / 34%), 0 0 0 12px rgb(245 158 11 / 14%); }
.text-anchor.translation-anchor.active { box-shadow: 0 0 0 6px rgb(37 99 235 / 34%), 0 0 0 12px rgb(37 99 235 / 14%); }
.note-marker { position: absolute; z-index: 5; width: 22px; height: 22px; padding: 0; transform: translate(-50%, -50%); border: 1px solid transparent; border-radius: 50%; color: white; font: 700 12px/20px sans-serif; box-shadow: 0 1px 3px rgb(0 0 0 / 22%); pointer-events: none; }
.note-marker::before { content: ""; position: absolute; inset: 2px; border-radius: 6px; border: 1px solid rgb(255 255 255 / 70%); pointer-events: none; }
.note-marker.translation-marker { width: 38px; height: 22px; border-radius: 5px; transform: translate(-50%, -50%); display: inline-grid; place-items: center; font: 700 13px/20px sans-serif; letter-spacing: 0; }
.note-marker.rail-marker { width: 30px; height: 28px; transform: translateY(0); border-radius: 9px; background: #f59e0b !important; border-color: #b7791f !important; color: #fff7ed !important; font: 800 12px/26px sans-serif; box-shadow: 0 4px 12px rgb(120 75 15 / 26%); }
.note-marker.rail-marker.translation-marker { width: 30px; height: 28px; transform: translateY(0); border-radius: 9px; background: #2563eb !important; border-color: #1d4ed8 !important; color: #eff6ff !important; font: 800 12px/26px sans-serif; box-shadow: 0 4px 12px rgb(29 78 216 / 26%); }
.note-marker.focused { outline: 3px solid rgb(245 158 11 / 34%); outline-offset: 3px; }
.note-marker.translation-marker.focused { outline-color: rgb(37 99 235 / 34%); }
.page-shell[data-active-tool='cursor'] .note-marker { pointer-events: auto; cursor: pointer; }
.page-shell[data-active-tool='cursor'] .note-marker.translation-marker { cursor: pointer; }
.page-shell[data-active-tool='cursor'] .note-marker:not(.rail-marker).translation-marker { cursor: grab; }
.page-shell[data-active-tool='cursor'] .note-marker:not(.rail-marker).translation-marker:active { cursor: grabbing; }
.selection-toolbar { position: absolute; z-index: 6; transform: translateX(-50%); height: 36px; display: flex; align-items: center; padding: 3px; border: 1px solid #c8c8c8; border-radius: 7px; background: rgb(250 250 250 / 98%); box-shadow: 0 5px 16px rgb(0 0 0 / 18%); }
.selection-toolbar button { height: 28px; padding: 0 9px; display: flex; align-items: center; gap: 6px; border: 0; border-radius: 5px; background: transparent; color: #3f4449; cursor: pointer; font-size: 12px; white-space: nowrap; }
.selection-toolbar button:hover { background: #e8e8e8; }
.selection-toolbar > span { width: 1px; height: 20px; background: #ddd; }
.translation-popup { position: absolute; z-index: 7; max-height: 260px; overflow: auto; padding: 10px 11px 11px; border: 1px solid #d8d8d8; border-radius: 9px; background: rgb(255 255 255 / 98%); box-shadow: 0 9px 24px rgb(0 0 0 / 18%); color: #3f4449; box-sizing: border-box; }
.translation-popup-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 7px; }
.translation-popup-header strong { font-size: 12px; color: #34383d; }
.translation-popup-header button { width: 22px; height: 22px; padding: 0; border: 0; border-radius: 5px; background: transparent; color: #777c81; cursor: pointer; }
.translation-popup-header button:hover { background: #ececec; color: #3f4449; }
.translation-loading, .translation-error, .translation-result { margin: 0; font-size: 12px; line-height: 1.6; }
.translation-loading { color: #6f757b; }
.translation-error { color: #a33d3d; }
.translation-result { max-height: 170px; overflow: auto; white-space: pre-wrap; padding-right: 3px; }
.save-translation-button { position: sticky; bottom: 0; margin-top: 8px; height: 28px; padding: 0 12px; border: 0; border-radius: 5px; background: #555b62; color: white; cursor: pointer; font-size: 12px; box-shadow: 0 -4px 10px rgb(255 255 255 / 84%); }
.note-editor { position: absolute; z-index: 7; width: 260px; padding: 13px; border-radius: 10px; background: #fffdf5; border: 1px solid #ead99c; box-shadow: 0 10px 30px rgb(0 0 0 / 20%); }
.note-editor-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin: -3px -3px 8px; padding: 3px; cursor: move; color: #713f12; }
.note-editor-header strong { display: block; color: #713f12; font-size: 13px; }
.note-editor-header i { color: #b78a35; font-size: 12px; }
.note-editor blockquote { max-height: 74px; overflow: auto; margin: 0 0 8px; padding: 7px 9px; border-left: 3px solid #d6b75d; border-radius: 5px; background: rgb(255 247 214 / 72%); color: #6b4e16; font-size: 12px; line-height: 1.45; }
.note-group { display: grid; gap: 6px; margin-top: 8px; }
.note-group > span { color: #8a6d21; font-size: 11px; font-weight: 700; }
.note-editor textarea { width: 100%; height: 92px; resize: vertical; border: 1px solid #e4d6ad; border-radius: 6px; padding: 8px; outline: none; color: #422006; background: white; font-size: 13px; }
.saved-translation { padding: 8px; border: 1px solid #ead99c; border-radius: 6px; background: white; }
.saved-translation small { display: block; margin-bottom: 5px; color: #9a7b34; font-size: 10px; }
.saved-translation p { max-height: 90px; overflow: auto; margin: 0; color: #422006; font-size: 12px; line-height: 1.55; white-space: pre-wrap; }
.note-editor-actions { display: flex; justify-content: flex-end; gap: 7px; margin-top: 8px; }
.note-editor button { border: 0; border-radius: 6px; padding: 6px 11px; background: #6a5b48; color: white; cursor: pointer; font-size: 12px; }
.note-editor button.secondary { background: transparent; color: #785b35; }
.empty-state { margin: 13vh auto 0; text-align: center; color: #70757a; max-width: 500px; }
.empty-icon { width: 76px; height: 76px; margin: 0 auto 20px; border-radius: 20px; background: #f7f7f7; border: 1px solid #d5d5d5; display: grid; place-items: center; font-size: 30px; color: #5f6469; box-shadow: 0 10px 30px rgb(0 0 0 / 6%); }
.empty-state h1 { margin: 0 0 10px; font-size: 23px; color: #33373c; }
.empty-state p { margin: 0 auto 24px; line-height: 1.75; font-size: 13px; color: #858a90; }
.empty-state button { height: 42px; padding: 0 18px; border: 1px solid #c9c9c9; background: #f8f8f8; color: #34383d; border-radius: 8px; cursor: pointer; }
.drop-hint { display: block; margin-top: 13px; font-size: 12px; color: #999da2; }
.drop-overlay { position: absolute; inset: 18px; z-index: 10; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; border: 2px dashed #85898e; border-radius: 16px; background: rgb(247 247 247 / 94%); color: #44494f; font-size: 18px; pointer-events: none; }
.loading { position: fixed; left: 50%; top: 82px; transform: translateX(-50%); z-index: 8; background: rgb(255 255 255 / 94%); border: 1px solid #d8d8d8; border-radius: 8px; padding: 9px 13px; font-size: 12px; color: #666b70; box-shadow: 0 3px 12px rgb(0 0 0 / 8%); display: flex; gap: 8px; align-items: center; }
.status-message { position: absolute; left: 50%; bottom: 70px; z-index: 15; transform: translateX(-50%); padding: 9px 14px; border-radius: 8px; background: rgb(55 58 62 / 94%); color: white; font-size: 12px; }
@media (max-width: 720px) { .page-stage { padding: 20px 74px 38px; } .page-flow { gap: 12px; } }
</style>
