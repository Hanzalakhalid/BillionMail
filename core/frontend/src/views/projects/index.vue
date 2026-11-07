<template>
	<div class="p-6">
		<div class="flex items-center justify-between gap-4 mb-5">
			<div><h1 class="text-xl font-semibold">Marketing projects</h1><p class="text-sm text-gray-500">Capture your goal, audience and requirements in a saved discovery workspace.</p></div>
			<n-button v-if="canManage" :disabled="busy" @click="newProject">New project</n-button>
		</div>
		<n-alert v-if="error" type="error" class="mb-4">{{ error }}</n-alert>
		<div class="grid grid-cols-1 lg:grid-cols-3 gap-5">
			<n-card title="Saved projects">
				<n-button :loading="loadingList" :disabled="busy" @click="loadList(false)">Refresh list</n-button>
				<n-empty v-if="!items.length && !loadingList" class="mt-4" description="No projects yet" />
				<button v-for="project in items" :key="project.id" class="block text-left w-full border-b py-4" :disabled="busy" @click="selectProject(project.id)">
					<span class="font-medium">{{ project.title }}</span><span class="block text-sm text-gray-500">{{ project.status }} · {{ project.missing.length }} unanswered</span>
				</button>
				<n-button v-if="cursor" class="mt-4" :loading="loadingList" @click="loadList(true)">Load more projects</n-button>
			</n-card>
			<div class="lg:col-span-2 space-y-5">
				<n-card :title="form.id ? form.title : 'New project'">
					<n-alert v-if="saved" type="success" class="mb-4">Requirements saved.</n-alert>
					<n-form label-placement="top" :disabled="!canManage || busy" @submit.prevent="save">
						<n-form-item label="Project title" required><n-input v-model:value="form.title" :maxlength="200" /></n-form-item>
						<n-form-item label="What do you want to achieve?" required><n-input v-model:value="form.objective" type="textarea" :maxlength="16000" /></n-form-item>
						<n-form-item label="Who is your audience?"><n-input v-model:value="form.requirements.audience" type="textarea" :maxlength="8000" /></n-form-item>
						<n-form-item label="Channels"><n-select v-model:value="form.requirements.channels" multiple :options="channelOptions" /></n-form-item>
						<n-form-item label="Languages"><n-select v-model:value="form.requirements.languages" multiple filterable tag :options="languageOptions" placeholder="en, ur, or another language tag" /></n-form-item>
						<n-form-item label="How will you measure success?"><n-input v-model:value="form.requirements.success_metric" :maxlength="2000" placeholder="For example, completed purchases" /></n-form-item>
						<n-form-item label="Constraints and preferences"><n-input v-model:value="form.requirements.constraints" type="textarea" :maxlength="8000" /></n-form-item>
						<n-button v-if="canManage" type="primary" attr-type="submit" :loading="saving">Save requirements</n-button>
					</n-form>
				</n-card>
				<n-card v-if="form.id" title="Project notes and history">
					<p v-if="missing.length" class="text-sm text-gray-500 mb-4">Still needed: {{ missing.map(field => fieldLabels[field] || field).join(', ') }}.</p>
					<div v-for="event in events" :key="event.id" class="border-b py-3">
						<p class="text-xs text-gray-500">{{ new Date(event.created_at).toLocaleString() }}</p>
						<p v-if="event.type === 'message.created'" class="whitespace-pre-wrap">{{ (event.payload as ProjectMessage).content }}</p>
						<p v-else class="text-sm">Requirements saved (revision {{ (event.payload as MarketingProject).revision }}).</p>
					</div>
					<n-button class="my-4" :loading="loadingEvents" :disabled="busy" @click="loadEvents">{{ moreEvents ? 'Load more history' : 'Refresh history' }}</n-button>
					<n-input v-if="canManage" v-model:value="note" type="textarea" :maxlength="16000" :disabled="busy" placeholder="Add context or a preference for this project" />
					<n-button v-if="canManage" class="mt-3" :disabled="busy || !note.trim()" :loading="sending" @click="sendNote">Save note</n-button>
				</n-card>
			</div>
		</div>
	</div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useCompanyStore } from '@/store'
import { addProjectMessage, getProject, listProjects, projectEvents, saveProject } from '@/api/modules/projects'
import type { MarketingProject, ProjectEvent, ProjectInput, ProjectMessage } from '@/api/modules/projects'

const companyStore = useCompanyStore()
const canManage = computed(() => ['owner', 'admin', 'manager'].includes(companyStore.activeCompany?.role || ''))
const empty = (): ProjectInput => ({ id: '', revision: 0, title: '', objective: '', requirements: { audience: '', channels: [], languages: [], success_metric: '', constraints: '' } })
const form = reactive<ProjectInput>(empty())
const items = ref<MarketingProject[]>([])
const cursor = ref('')
const events = ref<ProjectEvent[]>([])
const after = ref(0)
const moreEvents = ref(false)
const missing = ref<string[]>([])
const note = ref('')
const error = ref('')
const saved = ref(false)
const saving = ref(false)
const sending = ref(false)
const selecting = ref(false)
const loadingList = ref(false)
const loadingEvents = ref(false)
const busy = computed(() => saving.value || sending.value || selecting.value || loadingEvents.value)
let saveKey = crypto.randomUUID()
let noteKey = crypto.randomUUID()
const channelOptions = ['email', 'whatsapp', 'sms', 'push', 'social', 'blog', 'seo', 'landing_page'].map(value => ({ value, label: value.replace('_', ' ') }))
const languageOptions = ['en', 'ur', 'es', 'fr', 'de', 'ar'].map(value => ({ value, label: value }))
const fieldLabels: Record<string, string> = { audience: 'audience', channels: 'channels', languages: 'languages', success_metric: 'success measure' }
const fail = (err: unknown) => {
	if (err instanceof Error) error.value = err.message
	else if (err && typeof err === 'object' && 'msg' in err && typeof err.msg === 'string') error.value = err.msg
	else error.value = 'Could not complete the request. Try again.'
}
watch(form, () => { saveKey = crypto.randomUUID(); saved.value = false }, { deep: true })
watch(note, () => { noteKey = crypto.randomUUID() })
function newProject() {
	Object.assign(form, empty()); events.value = []; after.value = 0; moreEvents.value = false; missing.value = []; note.value = ''; error.value = ''
}
async function loadList(append: boolean) {
	if (loadingList.value) return
	loadingList.value = true
	try { const page = await listProjects(append ? cursor.value : ''); items.value = append ? [...items.value, ...page.items] : page.items; cursor.value = page.next_cursor } catch (err) { fail(err) } finally { loadingList.value = false }
}
async function loadEvents() {
	if (!form.id || loadingEvents.value) return
	const id = form.id
	loadingEvents.value = true
	try {
		const page = await projectEvents(id, after.value)
		if (form.id !== id) return
		events.value.push(...page.items); after.value = page.next_after; moreEvents.value = page.items.length === 100
	} catch (err) { fail(err) } finally { loadingEvents.value = false }
}
async function selectProject(id: string) {
	if (busy.value) return
	selecting.value = true; error.value = ''
	try {
		const project = await getProject(id)
		Object.assign(form, project); missing.value = project.missing; events.value = []; after.value = 0; note.value = ''; saved.value = false
		await loadEvents()
	} catch (err) { fail(err) } finally { selecting.value = false }
}
async function save() {
	if (busy.value || !canManage.value) return
	saving.value = true; error.value = ''
	try {
		const input: ProjectInput = { id: form.id, revision: form.revision, title: form.title, objective: form.objective, requirements: { ...form.requirements } }
		const project = await saveProject(input, saveKey)
		Object.assign(form, project); missing.value = project.missing
		await loadList(false); await loadEvents(); saved.value = true
	} catch (err) { fail(err) } finally { saving.value = false }
}
async function sendNote() {
	if (busy.value || !canManage.value || !note.value.trim()) return
	sending.value = true; error.value = ''
	try { await addProjectMessage(form.id, note.value, noteKey); note.value = ''; await loadEvents() } catch (err) { fail(err) } finally { sending.value = false }
}
onMounted(() => loadList(false))
</script>
