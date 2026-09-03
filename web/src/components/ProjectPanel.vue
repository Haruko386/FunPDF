<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  addFilesToAlbum,
  createAlbum,
  deleteAlbum,
  listAlbumFiles,
  listAlbums,
  removeFilesFromAlbum,
  updateAlbum,
} from '@/api/albums'
import {
  deleteFile,
  listFileAlbums,
  listFiles,
  updateFile,
  type CachedFile,
} from '@/api/files'
import { apiErrorMessage } from '@/api/http'
import type { Album } from '@/api/types'

type PanelView = 'albums' | 'library'
type ThumbnailCropTarget = 'create' | 'album'

const THUMBNAIL_WIDTH = 640
const THUMBNAIL_HEIGHT = 480

const view = ref<PanelView>('albums')
const albums = ref<Album[]>([])
const libraryFiles = ref<CachedFile[]>([])
const selectedAlbum = ref<Album | null>(null)
const albumFiles = ref<CachedFile[]>([])
const selectedFileIds = ref<string[]>([])
const assignmentTargets = ref<Record<string, string>>({})
const fileAlbums = ref<Record<string, Album[]>>({})
const loading = ref(false)
const actionBusy = ref(false)
const error = ref('')
const notice = ref('')
const showFilePicker = ref(false)
const editingAlbum = ref(false)
const albumDraft = ref({ name: '', description: '', thumbnail: '' })
const albumThumbnailName = ref('')
const expandedFileId = ref('')
const fileNameDrafts = ref<Record<string, string>>({})
const thumbnailFailures = ref<Record<string, boolean>>({})

const createOpen = ref(false)
const createName = ref('')
const createDescription = ref('')
const createThumbnail = ref('')
const createThumbnailName = ref('')
const createError = ref('')
const thumbnailCropOpen = ref(false)
const thumbnailCropTarget = ref<ThumbnailCropTarget>('create')
const thumbnailCropImage = ref('')
const thumbnailCropName = ref('')
const thumbnailCropScale = ref(1)
const thumbnailCropOffsetX = ref(0)
const thumbnailCropOffsetY = ref(0)
const thumbnailCropError = ref('')

const availableFiles = computed(() => {
  const linked = new Set(albumFiles.value.map(file => file.id))
  return libraryFiles.value.filter(file => !linked.has(file.id))
})

const thumbnailCropPreviewStyle = computed(() => ({
  objectPosition: `${50 + thumbnailCropOffsetX.value / 2}% ${50 + thumbnailCropOffsetY.value / 2}%`,
  transform: `scale(${thumbnailCropScale.value})`,
}))

function clearFeedback() {
  error.value = ''
  notice.value = ''
}

function formatSize(bytes: number) {
  if (!Number.isFinite(bytes) || bytes < 1) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const unit = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const value = bytes / 1024 ** unit
  return `${value >= 10 || unit === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`
}

function membershipsFor(fileId: string) {
  return fileAlbums.value[fileId] ?? []
}

function membershipSummary(fileId: string) {
  const memberships = membershipsFor(fileId)
  if (memberships.length === 0) return '不属于任何合集'
  return `已加入：${memberships.map(album => album.name).join('、')}`
}

function assignableAlbums(fileId: string) {
  const assignedIds = new Set(membershipsFor(fileId).map(album => album.id))
  return albums.value.filter(album => !assignedIds.has(album.id))
}

function firstAssignableAlbumId(fileId: string) {
  return assignableAlbums(fileId)[0]?.id ?? ''
}

function ensureAssignmentTarget(fileId: string) {
  if (assignmentTargets.value[fileId]) return
  assignmentTargets.value[fileId] = firstAssignableAlbumId(fileId)
}

async function loadMemberships(files: CachedFile[]) {
  const entries = await Promise.all(files.map(async file => [file.id, await listFileAlbums(file.id)] as const))
  fileAlbums.value = Object.fromEntries(entries)
}

function generatedThumbnail(name: string) {
  const canvas = document.createElement('canvas')
  canvas.width = THUMBNAIL_WIDTH
  canvas.height = THUMBNAIL_HEIGHT
  const context = canvas.getContext('2d')
  if (!context) return ''
  const gradient = context.createLinearGradient(0, 0, canvas.width, canvas.height)
  gradient.addColorStop(0, '#64748b')
  gradient.addColorStop(1, '#334155')
  context.fillStyle = gradient
  context.fillRect(0, 0, canvas.width, canvas.height)
  context.fillStyle = 'rgba(255, 255, 255, 0.94)'
  context.font = '600 72px system-ui, sans-serif'
  context.textAlign = 'center'
  context.textBaseline = 'middle'
  context.fillText(name.trim().slice(0, 2).toUpperCase() || 'PDF', canvas.width / 2, canvas.height / 2)
  return canvas.toDataURL('image/png')
}

function readFileAsDataURL(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })
}

function validateThumbnailFile(file: File, target: ThumbnailCropTarget) {
  if (!['image/png', 'image/jpeg', 'image/gif', 'image/webp'].includes(file.type)) {
    const message = '请选择 PNG、JPEG、GIF 或 WebP 图片'
    if (target === 'create') createError.value = message
    else error.value = message
    return false
  }
  if (file.size > 4 * 1024 * 1024) {
    const message = '封面图片不能超过 4 MB'
    if (target === 'create') createError.value = message
    else error.value = message
    return false
  }
  return true
}

async function openThumbnailCropper(file: File | undefined, target: ThumbnailCropTarget) {
  if (!file) return
  if (!validateThumbnailFile(file, target)) return
  createError.value = ''
  error.value = ''
  thumbnailCropTarget.value = target
  thumbnailCropImage.value = await readFileAsDataURL(file)
  thumbnailCropName.value = file.name
  thumbnailCropScale.value = 1
  thumbnailCropOffsetX.value = 0
  thumbnailCropOffsetY.value = 0
  thumbnailCropError.value = ''
  thumbnailCropOpen.value = true
}

function readImage(file?: File) {
  void openThumbnailCropper(file, 'create')
}

function readAlbumImage(file?: File) {
  void openThumbnailCropper(file, 'album')
}

function closeThumbnailCropper() {
  thumbnailCropOpen.value = false
  thumbnailCropError.value = ''
}

function loadImage(source: string) {
  return new Promise<HTMLImageElement>((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => reject(new Error('图片加载失败'))
    image.src = source
  })
}

async function renderCroppedThumbnail() {
  const image = await loadImage(thumbnailCropImage.value)
  const canvas = document.createElement('canvas')
  canvas.width = THUMBNAIL_WIDTH
  canvas.height = THUMBNAIL_HEIGHT
  const context = canvas.getContext('2d')
  if (!context) return ''

  const baseScale = Math.max(canvas.width / image.naturalWidth, canvas.height / image.naturalHeight)
  const drawScale = baseScale * thumbnailCropScale.value
  const drawWidth = image.naturalWidth * drawScale
  const drawHeight = image.naturalHeight * drawScale
  const maxOffsetX = Math.max((drawWidth - canvas.width) / 2, 0)
  const maxOffsetY = Math.max((drawHeight - canvas.height) / 2, 0)
  const drawX = (canvas.width - drawWidth) / 2 - maxOffsetX * thumbnailCropOffsetX.value / 100
  const drawY = (canvas.height - drawHeight) / 2 - maxOffsetY * thumbnailCropOffsetY.value / 100

  context.fillStyle = '#f1f2f3'
  context.fillRect(0, 0, canvas.width, canvas.height)
  context.drawImage(image, drawX, drawY, drawWidth, drawHeight)
  return canvas.toDataURL('image/png')
}

async function applyThumbnailCrop() {
  thumbnailCropError.value = ''
  try {
    const thumbnail = await renderCroppedThumbnail()
    if (!thumbnail) throw new Error('无法生成封面')
    if (thumbnailCropTarget.value === 'create') {
      createThumbnail.value = thumbnail
      createThumbnailName.value = thumbnailCropName.value
    } else {
      albumDraft.value.thumbnail = thumbnail
      albumThumbnailName.value = thumbnailCropName.value
    }
    closeThumbnailCropper()
  } catch (cropError) {
    console.error(cropError)
    thumbnailCropError.value = '无法裁剪这张图片'
  }
}

async function loadAll() {
  loading.value = true
  clearFeedback()
  try {
    const [nextAlbums, nextFiles] = await Promise.all([listAlbums(), listFiles()])
    albums.value = nextAlbums ?? []
    libraryFiles.value = nextFiles ?? []
    thumbnailFailures.value = {}
    await loadMemberships(libraryFiles.value)
    if (selectedAlbum.value) {
      selectedAlbum.value = albums.value.find(album => album.id === selectedAlbum.value?.id) ?? null
      if (selectedAlbum.value) albumFiles.value = await listAlbumFiles(selectedAlbum.value.id)
      else albumFiles.value = []
    }
  } catch (requestError) {
    error.value = apiErrorMessage(requestError, '无法读取文件库')
  } finally {
    loading.value = false
  }
}

async function openAlbum(album: Album) {
  selectedAlbum.value = { ...album }
  albumDraft.value = { name: album.name, description: album.description, thumbnail: album.thumbnail }
  albumThumbnailName.value = ''
  editingAlbum.value = false
  expandedFileId.value = ''
  albumFiles.value = []
  selectedFileIds.value = []
  showFilePicker.value = false
  clearFeedback()
  loading.value = true
  try {
    albumFiles.value = await listAlbumFiles(album.id) ?? []
  } catch (requestError) {
    error.value = apiErrorMessage(requestError, '无法读取合集文件')
  } finally {
    loading.value = false
  }
}

function closeAlbum() {
  selectedAlbum.value = null
  albumFiles.value = []
  showFilePicker.value = false
  selectedFileIds.value = []
  editingAlbum.value = false
  expandedFileId.value = ''
  clearFeedback()
}

function startAlbumEdit() {
  if (!selectedAlbum.value) return
  albumDraft.value = {
    name: selectedAlbum.value.name,
    description: selectedAlbum.value.description,
    thumbnail: selectedAlbum.value.thumbnail,
  }
  albumThumbnailName.value = ''
  editingAlbum.value = true
}

function cancelAlbumEdit() {
  editingAlbum.value = false
  if (selectedAlbum.value) {
    albumDraft.value = {
      name: selectedAlbum.value.name,
      description: selectedAlbum.value.description,
      thumbnail: selectedAlbum.value.thumbnail,
    }
  }
  albumThumbnailName.value = ''
}

function openCreate() {
  createName.value = ''
  createDescription.value = ''
  createThumbnail.value = ''
  createThumbnailName.value = ''
  createError.value = ''
  createOpen.value = true
}

function closeCreate() {
  if (!actionBusy.value) createOpen.value = false
}

async function submitCreate() {
  const name = createName.value.trim()
  if (!name) {
    createError.value = '请输入合集名称'
    return
  }
  actionBusy.value = true
  createError.value = ''
  try {
    const album = await createAlbum({
      name,
      description: createDescription.value.trim(),
      thumbnail: createThumbnail.value || generatedThumbnail(name),
    })
    albums.value = [album, ...albums.value]
    createOpen.value = false
    await openAlbum(album)
    notice.value = '合集已创建'
  } catch (requestError) {
    createError.value = apiErrorMessage(requestError, '创建合集失败')
  } finally {
    actionBusy.value = false
  }
}

async function saveAlbum() {
  if (!selectedAlbum.value || !albumDraft.value.name.trim()) return
  actionBusy.value = true
  clearFeedback()
  try {
    const payload = {
      name: albumDraft.value.name.trim(),
      description: albumDraft.value.description.trim(),
      thumbnail: albumDraft.value.thumbnail || generatedThumbnail(albumDraft.value.name.trim()),
    }
    await updateAlbum(selectedAlbum.value.id, payload)
    selectedAlbum.value = { ...selectedAlbum.value, ...payload }
    const index = albums.value.findIndex(album => album.id === selectedAlbum.value?.id)
    if (index >= 0) albums.value[index] = { ...selectedAlbum.value }
    editingAlbum.value = false
    notice.value = '合集信息已保存'
  } catch (requestError) {
    error.value = apiErrorMessage(requestError, '保存合集失败')
  } finally {
    actionBusy.value = false
  }
}

function openStoredFile(file: CachedFile) {
  window.dispatchEvent(new CustomEvent('funpdf:open-cached-file', { detail: { file } }))
}

function toggleFileActions(file: CachedFile) {
  expandedFileId.value = expandedFileId.value === file.id ? '' : file.id
  fileNameDrafts.value[file.id] = file.name
  if (expandedFileId.value === file.id) ensureAssignmentTarget(file.id)
}

async function saveFileName(file: CachedFile) {
  const name = fileNameDrafts.value[file.id]?.trim()
  if (!name || name === file.name) {
    expandedFileId.value = ''
    return
  }
  actionBusy.value = true
  clearFeedback()
  try {
    await updateFile(file.id, { name, mime_type: file.mime_type })
    const applyName = (item: CachedFile) => item.id === file.id ? { ...item, name } : item
    libraryFiles.value = libraryFiles.value.map(applyName)
    albumFiles.value = albumFiles.value.map(applyName)
    expandedFileId.value = ''
    notice.value = '文件名称已更新'
  } catch (requestError) {
    error.value = apiErrorMessage(requestError, '更新文件失败')
  } finally {
    actionBusy.value = false
  }
}

function markThumbnailFailed(fileId: string) {
  thumbnailFailures.value[fileId] = true
}

async function removeAlbum() {
  const album = selectedAlbum.value
  if (!album || !window.confirm(`删除合集“${album.name}”？公共文件不会被删除。`)) return
  actionBusy.value = true
  clearFeedback()
  try {
    await deleteAlbum(album.id)
    albums.value = albums.value.filter(item => item.id !== album.id)
    fileAlbums.value = Object.fromEntries(
      Object.entries(fileAlbums.value).map(([fileId, memberships]) => [
        fileId,
        memberships.filter(item => item.id !== album.id),
      ]),
    )
    closeAlbum()
    notice.value = '合集已删除，公共文件保持不变'
  } catch (requestError) {
    error.value = apiErrorMessage(requestError, '删除合集失败')
  } finally {
    actionBusy.value = false
  }
}

async function addSelectedFiles() {
  if (!selectedAlbum.value || selectedFileIds.value.length === 0) return
  actionBusy.value = true
  clearFeedback()
  try {
    const failed = await addFilesToAlbum(selectedAlbum.value.id, selectedFileIds.value)
    const failedIds = new Set(Object.keys(failed))
    const added = selectedFileIds.value.filter(id => !failedIds.has(id))
    albumFiles.value = [
      ...albumFiles.value,
      ...libraryFiles.value.filter(file => added.includes(file.id)),
    ]
    for (const fileId of added) {
      const memberships = membershipsFor(fileId)
      if (!memberships.some(album => album.id === selectedAlbum.value?.id)) {
        fileAlbums.value[fileId] = [...memberships, selectedAlbum.value]
      }
    }
    selectedFileIds.value = []
    showFilePicker.value = false
    notice.value = failedIds.size ? `已加入 ${added.length} 个文件，${failedIds.size} 个失败` : `已加入 ${added.length} 个文件`
  } catch (requestError) {
    error.value = apiErrorMessage(requestError, '加入合集失败')
  } finally {
    actionBusy.value = false
  }
}

async function detachFile(file: CachedFile) {
  if (!selectedAlbum.value) return
  actionBusy.value = true
  clearFeedback()
  try {
    await removeFilesFromAlbum(selectedAlbum.value.id, [file.id])
    albumFiles.value = albumFiles.value.filter(item => item.id !== file.id)
    fileAlbums.value[file.id] = membershipsFor(file.id).filter(album => album.id !== selectedAlbum.value?.id)
    notice.value = '已从合集中移除，公共文件仍然保留'
  } catch (requestError) {
    error.value = apiErrorMessage(requestError, '移除文件失败')
  } finally {
    actionBusy.value = false
  }
}

async function assignFile(file: CachedFile) {
  const albumId = assignmentTargets.value[file.id]
  if (!albumId) return
  actionBusy.value = true
  clearFeedback()
  try {
    const failed = await addFilesToAlbum(albumId, [file.id])
    if (failed[file.id]) throw new Error(failed[file.id])
    if (selectedAlbum.value?.id === albumId && !albumFiles.value.some(item => item.id === file.id)) {
      albumFiles.value.push(file)
    }
    const assignedAlbum = albums.value.find(album => album.id === albumId)
    if (assignedAlbum && !membershipsFor(file.id).some(album => album.id === albumId)) {
      fileAlbums.value[file.id] = [...membershipsFor(file.id), assignedAlbum]
    }
    assignmentTargets.value[file.id] = ''
    notice.value = `“${file.name}”已加入合集`
  } catch (requestError) {
    error.value = apiErrorMessage(requestError, '加入合集失败')
  } finally {
    actionBusy.value = false
  }
}

async function permanentlyDelete(file: CachedFile) {
  if (!window.confirm(`永久删除公共文件“${file.name}”？此操作会删除文件本体。`)) return
  actionBusy.value = true
  clearFeedback()
  try {
    await deleteFile(file.id)
    window.dispatchEvent(new CustomEvent('funpdf:cached-file-deleted', { detail: { fileId: file.id } }))
    libraryFiles.value = libraryFiles.value.filter(item => item.id !== file.id)
    albumFiles.value = albumFiles.value.filter(item => item.id !== file.id)
    delete fileAlbums.value[file.id]
    notice.value = '公共文件已永久删除'
  } catch (requestError) {
    error.value = apiErrorMessage(requestError, '永久删除文件失败')
  } finally {
    actionBusy.value = false
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && thumbnailCropOpen.value) closeThumbnailCropper()
  if (event.key === 'Escape' && createOpen.value) closeCreate()
}

onMounted(() => {
  void loadAll()
  window.addEventListener('funpdf:files-changed', loadAll)
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('funpdf:files-changed', loadAll)
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="project-panel">
    <div class="view-tabs" role="tablist" aria-label="文件管理区域">
      <button :class="{ active: view === 'albums' }" @click="view = 'albums'">
        <i class="fa-regular fa-folder-open"></i>合集
      </button>
      <button :class="{ active: view === 'library' }" @click="view = 'library'">
        <i class="fa-solid fa-box-archive"></i>公共文件
      </button>
    </div>

    <div v-if="loading" class="loading-row"><i class="fa-solid fa-circle-notch fa-spin"></i>正在读取…</div>
    <p v-if="error" class="feedback error"><i class="fa-solid fa-circle-exclamation"></i>{{ error }}</p>
    <p v-if="notice" class="feedback notice"><i class="fa-solid fa-circle-check"></i>{{ notice }}</p>

    <template v-if="view === 'albums'">
      <template v-if="!selectedAlbum">
        <div class="section-heading">
          <div><strong>我的合集</strong><small>{{ albums.length }} 个合集</small></div>
          <button class="primary compact" @click="openCreate"><i class="fa-solid fa-plus"></i>新建</button>
        </div>

        <button v-for="album in albums" :key="album.id" class="album-card" @click="openAlbum(album)">
          <span class="album-cover">
            <img v-if="album.thumbnail" :src="album.thumbnail" alt="" />
            <i v-else class="fa-regular fa-folder-open"></i>
          </span>
          <span class="album-copy"><strong>{{ album.name }}</strong><small>{{ album.description || '暂无描述' }}</small></span>
          <i class="fa-solid fa-chevron-right chevron"></i>
        </button>
        <div v-if="!loading && albums.length === 0" class="empty-state">
          <i class="fa-regular fa-folder-open"></i>
          <strong>还没有合集</strong>
          <span>创建合集来整理公共文件</span>
          <button class="primary" @click="openCreate">创建第一个合集</button>
        </div>
      </template>

      <template v-else>
        <button class="back-button" @click="closeAlbum"><i class="fa-solid fa-arrow-left"></i>返回合集</button>
        <div class="selected-header">
          <span class="selected-cover"><img :src="selectedAlbum.thumbnail" alt="" /></span>
          <div class="selected-copy"><strong>{{ selectedAlbum.name }}</strong><small>{{ albumFiles.length }} 个文件</small></div>
          <button v-if="!editingAlbum" class="edit-album-button" title="编辑合集信息" @click="startAlbumEdit">
            <i class="fa-regular fa-pen-to-square"></i>
          </button>
        </div>
        <p v-if="!editingAlbum" class="album-description">{{ selectedAlbum.description || '暂无描述' }}</p>
        <div v-else class="album-editor">
          <label class="cover-field">合集封面</label>
          <label class="cover-picker">
            <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" @change="readAlbumImage(($event.target as HTMLInputElement).files?.[0])" />
            <span class="cover-preview">
              <img v-if="albumDraft.thumbnail" :src="albumDraft.thumbnail" alt="封面预览" />
              <template v-else><i class="fa-regular fa-image"></i><small>未选择时自动生成</small></template>
            </span>
            <span class="cover-copy"><strong>{{ albumThumbnailName || '更换合集封面' }}</strong><small>PNG / JPEG / GIF / WebP，最大 4 MB</small></span>
          </label>
          <label>名称<input v-model="albumDraft.name" maxlength="80" /></label>
          <label>描述<textarea v-model="albumDraft.description" rows="2" maxlength="500"></textarea></label>
          <div class="button-row">
            <button class="secondary" :disabled="actionBusy" @click="cancelAlbumEdit">取消</button>
            <button class="primary" :disabled="actionBusy || !albumDraft.name.trim()" @click="saveAlbum">保存</button>
          </div>
          <button class="delete-album-link" :disabled="actionBusy" title="只删除合集，不删除公共文件" @click="removeAlbum">
            <i class="fa-regular fa-trash-can"></i>删除合集
          </button>
        </div>

        <div class="section-heading files-heading">
          <div><strong>合集文件</strong><small>移除不会删除文件本体</small></div>
          <button class="icon-action" title="从公共文件添加" @click="showFilePicker = !showFilePicker"><i class="fa-solid fa-plus"></i></button>
        </div>

        <div v-if="showFilePicker" class="file-picker">
          <label v-for="file in availableFiles" :key="file.id" class="check-row">
            <input v-model="selectedFileIds" type="checkbox" :value="file.id" />
            <span><strong>{{ file.name }}</strong><small>{{ formatSize(file.size) }}</small></span>
          </label>
          <div v-if="availableFiles.length === 0" class="mini-empty">所有公共文件都已在此合集中</div>
          <button class="primary picker-submit" :disabled="actionBusy || selectedFileIds.length === 0" @click="addSelectedFiles">
            加入选中的 {{ selectedFileIds.length }} 个文件
          </button>
        </div>

        <div v-for="file in albumFiles" :key="file.id" class="compact-file-card" :class="{ expanded: expandedFileId === file.id }">
          <div class="file-row">
            <button class="file-open" :title="`打开 ${file.name}`" @click="openStoredFile(file)">
              <span class="file-thumbnail">
                <img v-if="file.thumbnail && !thumbnailFailures[file.id]" :src="file.thumbnail" alt="PDF 第一页" @error="markThumbnailFailed(file.id)" />
                <i v-else class="fa-regular fa-file-pdf"></i>
              </span>
              <span class="file-copy"><strong>{{ file.name }}</strong><small>{{ formatSize(file.size) }}</small></span>
            </button>
            <button class="file-menu" :aria-expanded="expandedFileId === file.id" title="文件操作" @click.stop="toggleFileActions(file)">
              <i :class="expandedFileId === file.id ? 'fa-solid fa-chevron-up' : 'fa-solid fa-ellipsis' "></i>
            </button>
          </div>
          <Transition name="expand">
            <div v-if="expandedFileId === file.id" class="file-actions">
              <label>文件名<input v-model="fileNameDrafts[file.id]" maxlength="255" @keyup.enter="saveFileName(file)" /></label>
              <div class="file-action-buttons">
                <button @click="openStoredFile(file)"><i class="fa-regular fa-folder-open"></i>打开</button>
                <button class="primary" :disabled="actionBusy || !fileNameDrafts[file.id]?.trim()" @click="saveFileName(file)"><i class="fa-regular fa-floppy-disk"></i>保存</button>
                <button class="unlink" :disabled="actionBusy" title="文件仍会保留在公共文件区" @click="detachFile(file)"><i class="fa-solid fa-link-slash"></i>移出</button>
              </div>
            </div>
          </Transition>
        </div>
        <div v-if="!loading && albumFiles.length === 0" class="mini-empty">此合集暂无文件</div>
      </template>
    </template>

    <template v-else>
      <div class="section-heading">
        <div><strong>公共文件区</strong><small>{{ libraryFiles.length }} 个文件 · 删除会移除本体</small></div>
        <button class="icon-action" title="刷新" @click="loadAll"><i class="fa-solid fa-rotate"></i></button>
      </div>
      <div v-for="file in libraryFiles" :key="file.id" class="library-card">
        <div class="library-file">
          <button class="file-open" :title="`打开 ${file.name}`" @click="openStoredFile(file)">
            <span class="file-thumbnail">
              <img v-if="file.thumbnail && !thumbnailFailures[file.id]" :src="file.thumbnail" alt="PDF 第一页" @error="markThumbnailFailed(file.id)" />
              <i v-else class="fa-regular fa-file-pdf"></i>
            </span>
            <span class="file-copy"><strong>{{ file.name }}</strong><small>{{ formatSize(file.size) }} · {{ membershipSummary(file.id) }}</small></span>
          </button>
          <button class="file-menu" :aria-expanded="expandedFileId === file.id" title="展开文件操作" @click.stop="toggleFileActions(file)">
            <i :class="expandedFileId === file.id ? 'fa-solid fa-chevron-up' : 'fa-solid fa-ellipsis' "></i>
          </button>
        </div>
        <Transition name="expand">
          <div v-if="expandedFileId === file.id" class="file-actions library-actions">
            <label>文件名<input v-model="fileNameDrafts[file.id]" maxlength="255" @keyup.enter="saveFileName(file)" /></label>
            <div class="assign-row">
              <select v-model="assignmentTargets[file.id]" :title="membershipSummary(file.id)" aria-label="查看所属合集或选择新的合集">
                <option value="">{{ membershipSummary(file.id) }}</option>
                <option v-for="album in assignableAlbums(file.id)" :key="album.id" :value="album.id">加入：{{ album.name }}</option>
              </select>
              <button :disabled="actionBusy || !assignmentTargets[file.id]" title="加入所选合集" @click="assignFile(file)"><i class="fa-solid fa-arrow-right"></i></button>
            </div>
            <div class="file-action-buttons">
              <button @click="openStoredFile(file)"><i class="fa-regular fa-folder-open"></i>打开</button>
              <button class="primary" :disabled="actionBusy || !fileNameDrafts[file.id]?.trim()" @click="saveFileName(file)"><i class="fa-regular fa-floppy-disk"></i>保存</button>
              <button class="delete-file" :disabled="actionBusy" @click="permanentlyDelete(file)"><i class="fa-regular fa-trash-can"></i>删除</button>
            </div>
          </div>
        </Transition>
      </div>
      <div v-if="!loading && libraryFiles.length === 0" class="empty-state">
        <i class="fa-solid fa-box-open"></i>
        <strong>公共文件区为空</strong>
        <span>打开 PDF 后按 Ctrl+S 即可存入</span>
      </div>
    </template>
  </div>

  <Teleport to="body">
    <Transition name="modal">
      <div v-if="createOpen" class="modal-backdrop" role="presentation" @mousedown.self="closeCreate">
        <section class="create-modal" role="dialog" aria-modal="true" aria-labelledby="create-album-title">
          <header>
            <div><span class="modal-icon"><i class="fa-regular fa-folder-open"></i></span><div><h2 id="create-album-title">创建合集</h2><p>把相关 PDF 整理到一起</p></div></div>
            <button title="关闭" :disabled="actionBusy" @click="closeCreate"><i class="fa-solid fa-xmark"></i></button>
          </header>
          <div class="modal-body">
            <label>合集名称 <span>*</span><input v-model="createName" autofocus maxlength="80" placeholder="例如：毕业论文资料" @keyup.enter="submitCreate" /></label>
            <label>描述<textarea v-model="createDescription" rows="3" maxlength="500" placeholder="简单说明这个合集的用途（可选）"></textarea></label>
            <label class="cover-field">合集封面</label>
            <label class="cover-picker">
              <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" @change="readImage(($event.target as HTMLInputElement).files?.[0])" />
              <span class="cover-preview" :class="{ generated: !createThumbnail }">
                <img v-if="createThumbnail" :src="createThumbnail" alt="封面预览" />
                <template v-else><i class="fa-regular fa-image"></i><small>未选择时自动生成</small></template>
              </span>
              <span class="cover-copy"><strong>{{ createThumbnailName || '选择一张图片' }}</strong><small>PNG / JPEG / GIF / WebP，最大 4 MB</small></span>
            </label>
            <p v-if="createError" class="modal-error"><i class="fa-solid fa-circle-exclamation"></i>{{ createError }}</p>
          </div>
          <footer>
            <button class="secondary" :disabled="actionBusy" @click="closeCreate">取消</button>
            <button class="primary" :disabled="actionBusy || !createName.trim()" @click="submitCreate">
              <i :class="actionBusy ? 'fa-solid fa-circle-notch fa-spin' : 'fa-solid fa-plus'"></i>{{ actionBusy ? '创建中…' : '创建合集' }}
            </button>
          </footer>
        </section>
      </div>
    </Transition>
    <Transition name="modal">
      <div v-if="thumbnailCropOpen" class="modal-backdrop" role="presentation" @mousedown.self="closeThumbnailCropper">
        <section class="create-modal thumbnail-crop-modal" role="dialog" aria-modal="true" aria-labelledby="thumbnail-crop-title">
          <header>
            <div><span class="modal-icon"><i class="fa-regular fa-image"></i></span><div><h2 id="thumbnail-crop-title">调整封面</h2><p>裁剪为 4:3 封面</p></div></div>
            <button title="关闭" :disabled="actionBusy" @click="closeThumbnailCropper"><i class="fa-solid fa-xmark"></i></button>
          </header>
          <div class="modal-body">
            <div class="thumbnail-crop-stage">
              <img :src="thumbnailCropImage" alt="封面裁剪预览" :style="thumbnailCropPreviewStyle" />
              <span class="thumbnail-crop-frame"></span>
            </div>
            <label>缩放<input v-model.number="thumbnailCropScale" type="range" min="1" max="3" step="0.01" /></label>
            <label>横向位置<input v-model.number="thumbnailCropOffsetX" type="range" min="-100" max="100" step="1" /></label>
            <label>纵向位置<input v-model.number="thumbnailCropOffsetY" type="range" min="-100" max="100" step="1" /></label>
            <p v-if="thumbnailCropError" class="modal-error"><i class="fa-solid fa-circle-exclamation"></i>{{ thumbnailCropError }}</p>
          </div>
          <footer>
            <button class="secondary" :disabled="actionBusy" @click="closeThumbnailCropper">取消</button>
            <button class="primary" :disabled="actionBusy" @click="applyThumbnailCrop">使用封面</button>
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.project-panel { display: flex; flex-direction: column; gap: 9px; color: #3f4449; }
button { border: 0; border-radius: 7px; background: #e8e8e8; color: #454a50; cursor: pointer; }
button:hover:not(:disabled) { background: #dedede; }
button:disabled { opacity: .45; cursor: default; }
.primary { background: #4b535c; color: white; }.primary:hover:not(:disabled) { background: #343b43; }
.view-tabs { width: 100%; box-sizing: border-box; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); padding: 3px; border-radius: 9px; background: #ededed; }
.view-tabs button { min-width: 0; height: 32px; display: flex; align-items: center; justify-content: center; gap: 6px; background: transparent; font-size: 11px; white-space: nowrap; }
.view-tabs button.active { background: white; color: #262b30; box-shadow: 0 1px 4px rgb(0 0 0 / 9%); }
.loading-row { padding: 8px; display: flex; justify-content: center; gap: 7px; color: #7b8187; font-size: 11px; }
.feedback { margin: 0; padding: 8px 9px; display: flex; gap: 7px; border-radius: 7px; font-size: 11px; line-height: 1.45; }
.feedback.error { color: #953d3d; background: #f7eaea; }.feedback.notice { color: #356342; background: #eaf4ed; }
.section-heading { min-height: 38px; display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.section-heading > div { min-width: 0; }.section-heading strong, .section-heading small { display: block; }.section-heading strong { font-size: 12px; }.section-heading small { margin-top: 2px; color: #92979c; font-size: 9px; }
.compact { height: 29px; padding: 0 10px; font-size: 10px; }.compact i { margin-right: 5px; }
.album-card { width: 100%; min-height: 55px; padding: 7px; display: grid; grid-template-columns: 52px minmax(0, 1fr) 14px; align-items: center; gap: 9px; text-align: left; background: transparent; }
.album-card:hover { background: #ededed !important; }.album-cover, .selected-cover { overflow: hidden; display: grid; place-items: center; background: #e6e8ea; color: #70777e; }
.album-cover { width: 52px; aspect-ratio: 4 / 3; border-radius: 8px; }.album-cover img, .selected-cover img { width: 100%; height: 100%; object-fit: cover; }
.album-copy, .file-copy { min-width: 0; }.album-copy strong, .album-copy small, .file-copy strong, .file-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.album-copy strong, .file-copy strong { font-size: 11px; }.album-copy small, .file-copy small { margin-top: 4px; color: #92979c; font-size: 9px; }.chevron { color: #a2a6aa; font-size: 9px; }
.empty-state { min-height: 170px; padding: 22px 12px; display: flex; flex-direction: column; align-items: center; justify-content: center; color: #92979c; text-align: center; }
.empty-state > i { margin-bottom: 12px; font-size: 28px; color: #b1b5b9; }.empty-state strong { color: #565c62; font-size: 12px; }.empty-state span { margin: 5px 0 14px; font-size: 10px; }.empty-state button { min-height: 31px; padding: 0 12px; font-size: 10px; }
.back-button { align-self: flex-start; padding: 6px 8px; background: transparent; color: #6c7278; font-size: 10px; }.back-button i { margin-right: 6px; }
.selected-header { padding: 3px 0 6px; display: flex; align-items: center; gap: 10px; }.selected-cover { width: 64px; aspect-ratio: 4 / 3; flex: 0 0 64px; border-radius: 9px; }.selected-copy { min-width: 0; flex: 1; }.selected-header strong, .selected-header small { display: block; }.selected-header strong { overflow: hidden; color: #353a3f; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }.selected-header small { margin-top: 4px; color: #92979c; font-size: 10px; }.edit-album-button { width: 32px; height: 32px; flex: 0 0 32px; background: transparent; color: #66717c; font-size: 14px; }
.album-description { margin: 0; padding: 9px 10px; border-left: 2px solid #d9dcdf; color: #73797f; font-size: 10px; line-height: 1.6; white-space: pre-wrap; }.album-editor { display: grid; gap: 8px; padding: 9px; border: 1px solid #e1e2e3; border-radius: 9px; background: #f5f5f5; }.delete-album-link { justify-self: start; padding: 5px 3px; background: transparent; color: #a14a4a; font-size: 9px; }.delete-album-link i { margin-right: 5px; }
label { display: grid; gap: 4px; color: #777c81; font-size: 10px; }
input, textarea, select { width: 100%; border: 1px solid #d8dadd; border-radius: 7px; padding: 8px; background: white; color: #40454a; outline: none; font-size: 11px; }
input:focus, textarea:focus, select:focus { border-color: #9299a1; box-shadow: 0 0 0 2px rgb(100 116 139 / 10%); } textarea { resize: vertical; }
.button-row { display: grid; grid-template-columns: 1fr 1fr; gap: 7px; }.button-row button { min-height: 32px; padding: 0 10px; font-size: 10px; }
.files-heading { margin-top: 5px; border-top: 1px solid #e6e7e8; padding-top: 8px; }.icon-action { width: 29px; height: 29px; flex: 0 0 29px; }
.file-picker { padding: 8px; display: grid; gap: 5px; border: 1px solid #dedfe1; border-radius: 9px; background: #f4f4f4; }
.check-row { grid-template-columns: 16px minmax(0, 1fr); align-items: center; padding: 5px; border-radius: 5px; background: white; }.check-row input { width: 14px; height: 14px; margin: 0; }.check-row strong, .check-row small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.check-row strong { color: #4b5055; font-size: 10px; }.check-row small { margin-top: 2px; font-size: 8px; }
.picker-submit { min-height: 30px; font-size: 10px; }.mini-empty { padding: 11px 5px; color: #969ba0; text-align: center; font-size: 10px; }
.compact-file-card, .library-card { overflow: hidden; border-bottom: 1px solid #e8e9ea; background: transparent; transition: background .16s ease, border-color .16s ease; }.compact-file-card.expanded, .library-card:has(.file-actions) { margin: 2px 0; border: 1px solid #dde0e2; border-radius: 9px; background: white; }
.file-row, .library-file { min-height: 50px; display: grid; grid-template-columns: minmax(0, 1fr) 30px; align-items: center; gap: 4px; }.file-open { min-width: 0; min-height: 48px; padding: 5px 2px; display: grid; grid-template-columns: 38px minmax(0, 1fr); align-items: center; gap: 8px; text-align: left; background: transparent; }.file-open:hover:not(:disabled) { background: #f1f1f1; }
.file-thumbnail { width: 38px; height: 38px; overflow: hidden; display: grid; place-items: center; border: 1px solid #e4dddd; border-radius: 7px; background: #f5eded; color: #9a5656; font-size: 15px; }.file-thumbnail img { width: 100%; height: 100%; display: block; object-fit: cover; object-position: top center; }.file-menu { width: 28px; height: 28px; padding: 0; background: transparent; color: #778089; }.file-menu[aria-expanded='true'] { background: #e8e9ea; color: #3f474f; }
.file-actions { padding: 8px; display: grid; gap: 7px; border-top: 1px solid #e8e9ea; background: #f7f7f7; }.file-actions input { height: 30px; padding: 0 7px; }.file-action-buttons { display: grid; grid-template-columns: repeat(3, 1fr); gap: 5px; }.file-action-buttons button { min-width: 0; height: 29px; padding: 0 4px; font-size: 9px; white-space: nowrap; }.file-action-buttons i { margin-right: 4px; }.file-action-buttons .unlink { color: #7b5d42; background: #f1ebe5; }.file-action-buttons .delete-file { color: #a04444; background: #f3e7e7; }
.library-card { padding: 0 2px; }.library-actions { margin: 0 -2px; }.assign-row { display: grid; grid-template-columns: minmax(0, 1fr) 30px; gap: 5px; }.assign-row select { height: 30px; padding: 0 6px; font-size: 9px; }.assign-row button { width: 30px; height: 30px; }
.expand-enter-active, .expand-leave-active { transition: opacity .14s ease, max-height .18s ease; overflow: hidden; }.expand-enter-from, .expand-leave-to { max-height: 0; opacity: 0; }.expand-enter-to, .expand-leave-from { max-height: 150px; opacity: 1; }
.modal-backdrop { position: fixed; inset: 0; z-index: 1000; display: grid; place-items: center; padding: 20px; background: rgb(24 29 35 / 38%); backdrop-filter: blur(8px); -webkit-backdrop-filter: blur(8px); }
.create-modal { width: min(440px, 100%); overflow: hidden; border: 1px solid rgb(255 255 255 / 65%); border-radius: 16px; background: #fafafa; box-shadow: 0 24px 70px rgb(15 23 42 / 28%); }
.create-modal header { min-height: 76px; padding: 15px 17px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #e7e7e7; }.create-modal header > div { display: flex; align-items: center; gap: 11px; }.create-modal header > button { width: 32px; height: 32px; background: transparent; }
.modal-icon { width: 40px; height: 40px; display: grid; place-items: center; border-radius: 11px; background: #e8eaed; color: #4f5862; }.create-modal h2 { margin: 0; color: #30353a; font-size: 16px; }.create-modal header p { margin: 4px 0 0; color: #8b9095; font-size: 10px; }
.modal-body { padding: 18px; display: grid; gap: 13px; }.modal-body label { gap: 6px; color: #555b61; font-size: 11px; font-weight: 600; }.modal-body label > span { color: #a54444; }.modal-body input, .modal-body textarea { padding: 10px; font-size: 12px; font-weight: 400; }
.cover-field { margin-bottom: -7px; }.cover-picker { grid-template-columns: 96px minmax(0, 1fr); align-items: center; cursor: pointer; }.cover-picker > input { display: none; }.cover-preview { width: 96px; aspect-ratio: 4 / 3; overflow: hidden; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 3px; border: 1px dashed #c8cbd0; border-radius: 8px; background: #f1f2f3; color: #8b9197; }.cover-preview img { width: 100%; height: 100%; object-fit: cover; }.cover-preview i { font-size: 15px; }.cover-preview small { font-size: 7px; font-weight: 400; }.cover-copy strong, .cover-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.cover-copy strong { color: #4c5258; font-size: 11px; }.cover-copy small { margin-top: 5px; color: #969ba0; font-size: 9px; font-weight: 400; }
.thumbnail-crop-modal { width: min(560px, 100%); }
.thumbnail-crop-stage { position: relative; aspect-ratio: 4 / 3; overflow: hidden; border-radius: 12px; background: #111827; }
.thumbnail-crop-stage img { width: 100%; height: 100%; display: block; object-fit: cover; transition: transform .12s ease, object-position .12s ease; transform-origin: center; }
.thumbnail-crop-frame { position: absolute; inset: 0; border: 2px solid rgb(255 255 255 / 86%); border-radius: 12px; box-shadow: inset 0 0 0 999px rgb(15 23 42 / 10%); pointer-events: none; }
.thumbnail-crop-modal input[type='range'] { padding: 0; accent-color: #4b535c; }
.modal-error { margin: 0; display: flex; gap: 7px; color: #a13e3e; font-size: 10px; }.create-modal footer { padding: 13px 18px; display: flex; justify-content: flex-end; gap: 8px; border-top: 1px solid #e7e7e7; background: #f5f5f5; }.create-modal footer button { min-width: 82px; height: 34px; padding: 0 13px; font-size: 11px; }.create-modal footer i { margin-right: 6px; }.secondary { background: #e5e5e5; }
.modal-enter-active, .modal-leave-active { transition: opacity .18s ease; }.modal-enter-active .create-modal, .modal-leave-active .create-modal { transition: transform .18s ease, opacity .18s ease; }.modal-enter-from, .modal-leave-to { opacity: 0; }.modal-enter-from .create-modal, .modal-leave-to .create-modal { opacity: 0; transform: translateY(10px) scale(.98); }
@media (max-width: 520px) { .modal-backdrop { padding: 12px; }.create-modal { border-radius: 13px; }.modal-body { padding: 15px; } }
</style>
