export interface DesktopEnvironment {
  buildType: string
  platform: string
  arch: string
}

type WailsRuntime = {
  BrowserOpenURL?: (url: string) => void
  CanResolveFilePaths?: () => boolean
  Environment?: () => Promise<DesktopEnvironment>
  OnFileDrop?: (callback: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean) => void
  OnFileDropOff?: () => void
}

declare global {
  interface Window {
    runtime?: WailsRuntime
  }
}

export function isDesktopRuntimeAvailable() {
  return typeof window !== 'undefined' && typeof window.runtime?.Environment === 'function'
}

export async function getDesktopEnvironment() {
  if (!isDesktopRuntimeAvailable()) return null
  return window.runtime?.Environment?.() ?? null
}

export function openExternalURL(url: string) {
  if (typeof window.runtime?.BrowserOpenURL === 'function') {
    window.runtime.BrowserOpenURL(url)
    return true
  }
  return false
}

export function onDesktopFileDrop(callback: (paths: string[]) => void) {
  if (typeof window.runtime?.OnFileDrop !== 'function') return null

  window.runtime.OnFileDrop((_x, _y, paths) => {
    const pdfPaths = paths.filter(path => /\.pdf$/i.test(path))
    if (pdfPaths.length > 0) callback(pdfPaths)
  }, false)

  return () => {
    window.runtime?.OnFileDropOff?.()
  }
}
