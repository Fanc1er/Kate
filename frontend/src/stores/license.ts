import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as licenseApi from '../api/license'

export const useLicenseStore = defineStore('license', () => {
  const status = ref('missing')
  const machineCode = ref('')
  const source = ref('')
  const daysRemaining = ref(0)
  const notBefore = ref('')
  const notAfter = ref('')
  const maxAssets = ref(0)
  const maxWorkers = ref(0)
  const loaded = ref(false)

  async function fetchStatus(): Promise<void> {
    const s = await licenseApi.status()
    status.value = s.status
    daysRemaining.value = s.days_remaining
    notBefore.value = s.not_before
    notAfter.value = s.not_after
    maxAssets.value = s.max_assets
    maxWorkers.value = s.max_workers
    loaded.value = true
  }

  async function fetchMachineCode(): Promise<void> {
    const m = await licenseApi.machineCode()
    machineCode.value = m.machine_code
    source.value = m.source
  }

  async function importLicense(content: string): Promise<void> {
    await licenseApi.importLicense(content)
    await fetchStatus()
  }

  return {
    status,
    machineCode,
    source,
    daysRemaining,
    notBefore,
    notAfter,
    maxAssets,
    maxWorkers,
    loaded,
    fetchStatus,
    fetchMachineCode,
    importLicense,
  }
})
