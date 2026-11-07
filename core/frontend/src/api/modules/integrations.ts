import { instance } from '@/api'

export interface IntegrationConnection {
 id: string
 provider: string
 capability: string
 name: string
 status: 'unverified' | 'connected' | 'degraded' | 'disconnected'
 configuration: Record<string, unknown>
 last_verified_at: string | null
 last_error: string | null
}

export interface IntegrationInput {
 provider: string
 capability: string
 name: string
 configuration: Record<string, unknown>
 secret: string
}

export const listIntegrations = () => instance.get<IntegrationConnection[]>('/v1/integrations/list')
export const createIntegration = (input: IntegrationInput) =>
 instance.post<{ id: string }>('/v1/integrations/create', input)
export const rotateIntegration = (id: string, secret: string) =>
 instance.post('/v1/integrations/rotate', { id, secret })
export const disconnectIntegration = (id: string) =>
 instance.post('/v1/integrations/disconnect', { id })
