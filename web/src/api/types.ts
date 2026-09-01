export interface BaseModel {
  created_at?: string
  updated_at?: string
  create_time?: number
  create_date?: string
  update_time?: number
  update_date?: string
}

export interface Album extends BaseModel {
  id: string
  name: string
  thumbnail: string
  description: string
}

export interface Provider extends BaseModel {
  id: string
  name: string
  enabled?: boolean
  base_url?: string
  url?: string
  url_suffix?: Record<string, string>
  models?: ProviderModel[]
}

export interface ProviderModel extends BaseModel {
  id: string
  name: string
  provider_name?: string
}

export interface RuntimeInfo {
  mode: string
  version: string
  database: string
  database_path?: string
  cache_dir: string
}
