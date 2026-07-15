import { deleteJSONWithResponse, getJSON, patchJSON, postJSON } from './client'

export type ManagedAgent = {
  id: string
  user_id: string
  agent_id: string
  name: string
  purpose: string
  status: 'active' | 'disabled'
  api_token_configured: boolean
  api_token_hint?: string
  api_token_created_at?: string
  last_used_at?: string
  created_at: string
  updated_at: string
}

export type CreateAgentInput = {
  agent_id: string
  name: string
  purpose: string
}

type AgentListResponse = { items: ManagedAgent[] }
export type GeneratedAgentToken = { token: string; agent: ManagedAgent }

export type AgentSummary = {
  id: string
  agent_id: string
  name: string
  purpose: string
  status: 'active' | 'disabled'
}

type AgentSummaryListResponse = { items: AgentSummary[] }

function agentPath(userID: string, suffix = '') {
  return `/api/v1/admin/users/${encodeURIComponent(userID)}/agents${suffix}`
}

export async function listAgents(userID: string): Promise<ManagedAgent[]> {
  const response = await getJSON<AgentListResponse>(agentPath(userID))
  return response.items
}

export async function listCurrentAgents(): Promise<AgentSummary[]> {
  const response = await getJSON<AgentSummaryListResponse>('/api/v1/agents')
  return response.items
}

export function createAgent(userID: string, input: CreateAgentInput): Promise<GeneratedAgentToken> {
  return postJSON<GeneratedAgentToken>(agentPath(userID), input)
}

export function rotateAgentToken(userID: string, agentID: string): Promise<GeneratedAgentToken> {
  return postJSON<GeneratedAgentToken>(agentPath(userID, `/${encodeURIComponent(agentID)}/api-token`), {})
}

export function revokeAgentToken(userID: string, agentID: string): Promise<ManagedAgent> {
  return deleteJSONWithResponse<ManagedAgent>(agentPath(userID, `/${encodeURIComponent(agentID)}/api-token`))
}

export function updateAgentStatus(userID: string, agent: ManagedAgent, status: ManagedAgent['status']): Promise<ManagedAgent> {
  return patchJSON<ManagedAgent>(agentPath(userID, `/${encodeURIComponent(agent.id)}`), {
    name: agent.name,
    purpose: agent.purpose,
    status
  })
}
