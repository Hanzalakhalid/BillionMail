<template>
 <div class="p-6">
  <div class="flex justify-between items-center mb-5">
   <div>
    <h1 class="text-xl font-semibold">Integrations</h1>
    <p class="text-sm text-gray-500">Connect providers for future marketing projects. A saved key remains unverified until a provider test succeeds.</p>
   </div>
   <n-button v-if="canManage" type="primary" @click="showCreate = true">Add integration</n-button>
  </div>
  <n-alert v-if="error" type="error" class="mb-4">{{ error }}</n-alert>
  <n-spin :show="loading">
   <n-empty v-if="!connections.length" description="No integrations connected" />
   <n-card v-for="connection in connections" :key="connection.id" class="mb-3">
    <div class="flex justify-between items-center">
     <div>
      <div class="font-medium">{{ connection.name }}</div>
      <div class="text-sm text-gray-500">{{ connection.provider }} · {{ connection.capability }}</div>
      <div v-if="connection.last_error" class="text-sm text-red-600">{{ connection.last_error }}</div>
     </div>
     <div class="flex items-center gap-3">
      <n-tag :type="connection.status === 'connected' ? 'success' : 'warning'">{{ connection.status }}</n-tag>
      <n-button v-if="canManage" size="small" @click="openRotate(connection)">Rotate key</n-button>
      <n-popconfirm v-if="canManage" @positive-click="disconnect(connection.id)">
       <template #trigger><n-button size="small" type="error" ghost>Disconnect</n-button></template>
       Disconnect this integration and revoke its credential?
      </n-popconfirm>
     </div>
    </div>
   </n-card>
  </n-spin>

  <n-modal v-model:show="showCreate" preset="card" title="Add integration" style="width: 520px">
   <n-form label-placement="top" @submit.prevent="create">
    <n-form-item label="Provider"><n-input v-model:value="form.provider" placeholder="openai or runway" /></n-form-item>
    <n-form-item label="Capability"><n-select v-model:value="form.capability" :options="capabilityOptions" /></n-form-item>
    <n-form-item label="Name"><n-input v-model:value="form.name" placeholder="Primary image provider" /></n-form-item>
    <n-form-item label="API key"><n-input v-model:value="form.secret" type="password" show-password-on="click" autocomplete="new-password" /></n-form-item>
    <p class="text-sm text-gray-500 mb-4">The key is encrypted on the server and is never returned here.</p>
    <n-button type="primary" :loading="saving" :disabled="!canCreate" @click="create">Save integration</n-button>
   </n-form>
  </n-modal>
  <n-modal v-model:show="showRotate" preset="card" title="Rotate API key" style="width: 520px">
   <n-form-item label="New API key"><n-input v-model:value="newSecret" type="password" show-password-on="click" autocomplete="new-password" /></n-form-item>
   <n-button type="primary" :loading="saving" :disabled="!newSecret" @click="rotate">Rotate key</n-button>
  </n-modal>
 </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { createIntegration, disconnectIntegration, listIntegrations, rotateIntegration, type IntegrationConnection } from '@/api/modules/integrations'
import { useCompanyStore } from '@/store'

const companyStore = useCompanyStore()
const canManage = computed(() => ['owner', 'admin'].includes(companyStore.activeCompany?.role || ''))

const capabilityOptions = ['text', 'image', 'video', 'email', 'whatsapp', 'sms', 'push', 'commerce', 'social'].map(value => ({ label: value, value }))
const connections = ref<IntegrationConnection[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const showCreate = ref(false)
const showRotate = ref(false)
const rotatingId = ref('')
const newSecret = ref('')
const form = reactive({ provider: '', capability: 'image', name: '', secret: '' })
const canCreate = computed(() => Boolean(form.provider.trim() && form.capability && form.name.trim() && form.secret))

async function refresh() {
 loading.value = true
 error.value = ''
 try { connections.value = await listIntegrations() as unknown as IntegrationConnection[] }
 catch (cause) { error.value = String(cause) }
 finally { loading.value = false }
}
async function create() {
 if (!canCreate.value) return
 saving.value = true
 error.value = ''
 try {
  await createIntegration({ ...form, configuration: {} })
  form.secret = ''
  showCreate.value = false
  await refresh()
 } catch (cause) { error.value = String(cause) }
 finally { saving.value = false }
}
function openRotate(connection: IntegrationConnection) { rotatingId.value = connection.id; newSecret.value = ''; showRotate.value = true }
async function rotate() {
 if (!newSecret.value) return
 saving.value = true
 try { await rotateIntegration(rotatingId.value, newSecret.value); newSecret.value = ''; showRotate.value = false; await refresh() }
 catch (cause) { error.value = String(cause) }
 finally { saving.value = false }
}
async function disconnect(id: string) {
 try { await disconnectIntegration(id); await refresh() }
 catch (cause) { error.value = String(cause) }
}
onMounted(refresh)
</script>
