import { get, post, put, del } from './http'

export interface Scenario {
  id: number
  name: string
  scenario_type: string
  description: string
  policy_id: number
  asset_group_name: string
  activated: boolean
  activated_at?: string
  deactivated_at?: string
  created_at: string
}

export interface ScenarioInput {
  name: string
  scenario_type: string
  description: string
  policy_id: number
  asset_group_name: string
}

export function listScenarios(): Promise<Scenario[]> {
  return get('/scenarios')
}

export function createScenario(input: ScenarioInput & { activated?: boolean }): Promise<Scenario> {
  return post('/scenarios', input)
}

export function updateScenario(id: number, patch: Partial<ScenarioInput>): Promise<Scenario> {
  return put(`/scenarios/${id}`, patch)
}

export function deleteScenario(id: number): Promise<void> {
  return del(`/scenarios/${id}`)
}

export function toggleScenario(id: number, activated: boolean): Promise<Scenario> {
  return post(`/scenarios/${id}/toggle`, { activated })
}
