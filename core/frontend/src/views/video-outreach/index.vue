<template>
	<div class="p-24px">
		<n-grid :cols="6" :x-gap="12" class="mb-16px" responsive="screen" item-responsive>
			<n-gi v-for="card in statCards" :key="card.label" span="6 m:3 l:1">
				<n-card size="small">
					<n-statistic :label="card.label" :value="card.value" />
				</n-card>
			</n-gi>
		</n-grid>
		<bt-table-layout>
			<template #toolsLeft>
				<n-button type="primary" @click="openImport">Import leads (CSV)</n-button>
			</template>
			<template #toolsRight>
				<n-select
					v-model:value="tableParams.tier"
					:options="tierOptions"
					:placeholder="'Filter by tier'"
					clearable
					style="width: 160px; margin-right: 12px"
					@update:value="() => fetchTable(true)"
				/>
			</template>
			<template #table>
				<n-data-table v-bind="tableProps" :columns="columns" :row-key="(row: any) => row.contact_email">
					<template #empty>
						<n-empty description="No leads yet. Import a CSV from the scraper to score them." />
					</template>
				</n-data-table>
			</template>
			<template #pageRight>
				<bt-table-page v-bind="pageProps" @refresh="fetchTable"> </bt-table-page>
			</template>
		</bt-table-layout>

		<n-modal v-model:show="importForm.show" preset="card" title="Import leads" :style="{ width: '560px' }">
			<n-alert type="info" class="mb-16px">
				Upload a Google Maps CSV (e.g. from the scraper kit). Needed column: Email. Used for the score: Rating,
				ReviewCount, Website, Phone. Rows without an email are skipped.
			</n-alert>
			<n-form label-placement="top">
				<n-form-item label="Contact group">
					<n-select v-model:value="importForm.groupId" :options="groupOptions" placeholder="Select a group" filterable />
				</n-form-item>
				<n-form-item label="CSV file">
					<input type="file" accept=".csv,text/csv" @change="handleFile" />
				</n-form-item>
			</n-form>
			<n-alert v-if="importResult" type="success" class="mb-16px">
				Imported {{ importResult.imported }} (skipped {{ importResult.skipped }}): Tier 1:
				{{ importResult.tier_1 }}, Tier 2: {{ importResult.tier_2 }}, Skip: {{ importResult.tier_3 }}
			</n-alert>
			<n-button type="primary" :disabled="!importForm.groupId || !importForm.content" @click="handleImport">
				Import and score
			</n-button>
		</n-modal>

		<n-modal v-model:show="videoForm.show" preset="card" title="Add video" :style="{ width: '560px' }">
			<n-alert type="info" class="mb-16px">
				Record a short video (phone, Loom) and paste a direct link to the video file (.mp4).
			</n-alert>
			<n-form label-placement="top">
				<n-form-item label="Contact">
					<n-input :value="videoForm.email" disabled />
				</n-form-item>
				<n-form-item label="Video link (.mp4)">
					<n-input v-model:value="videoForm.videoUrl" placeholder="https://.../video.mp4" />
				</n-form-item>
				<n-form-item label="Preview image (optional)">
					<n-input v-model:value="videoForm.thumbnailUrl" placeholder="https://.../thumbnail.jpg" />
				</n-form-item>
				<n-form-item label="Book a Call link (optional)">
					<n-input v-model:value="videoForm.ctaUrl" placeholder="https://cal.com/your-name" />
				</n-form-item>
			</n-form>
			<n-button type="primary" :disabled="!videoForm.videoUrl.trim()" @click="handleSaveVideo">Save</n-button>
		</n-modal>
	</div>
</template>

<script lang="tsx" setup>
import { DataTableColumns, NTag, NButton, SelectOption } from 'naive-ui'
import { useDataTable } from '@/hooks/useDataTable'
import { isObject } from '@/utils'
import { getGroupAll } from '@/api/modules/contacts/group'
import { formatTime } from '@/utils/time'
import {
	listVideoOutreach,
	importLeads,
	setVideo,
	getVideoOutreachStats,
	type VideoStatus,
	type VideoOutreachStats,
} from '@/api/modules/video-outreach'

// Landing page activity totals
const stats = ref<VideoOutreachStats | null>(null)

const statCards = computed(() => [
	{ label: 'Videos ready', value: stats.value?.ready_videos ?? 0 },
	{ label: 'Page opens', value: stats.value?.views ?? 0 },
	{ label: 'Video plays', value: stats.value?.plays ?? 0 },
	{ label: 'Book a Call clicks', value: stats.value?.clicks ?? 0 },
	{ label: 'Leads who opened', value: stats.value?.leads_viewed ?? 0 },
	{ label: 'Leads who clicked', value: stats.value?.leads_clicked ?? 0 },
])

const loadStats = async () => {
	const res = await getVideoOutreachStats()
	if (isObject<VideoOutreachStats>(res)) {
		stats.value = res
	}
}

// Fetch the list and refresh the cards together
const listWithStats = (params: any) => {
	loadStats()
	return listVideoOutreach(params)
}

const tierOptions = [
	{ label: 'Tier 1 (Video)', value: 'tier_1' },
	{ label: 'Tier 2 (Text)', value: 'tier_2' },
	{ label: 'Tier 3 (Skip)', value: 'tier_3' },
]

const { tableProps, pageProps, tableParams, fetchTable } = useDataTable<any>({
	params: {
		page: 1,
		page_size: 20,
		tier: null as string | null,
	},
	fetchFn: listWithStats,
})

// Landing links are saved with the public base URL; open them on the current host so they work locally too
const openLanding = (url: string) => {
	const parsed = new URL(url, window.location.origin)
	window.open(window.location.origin + parsed.pathname + parsed.search, '_blank')
}

// Import
const groupOptions = ref<SelectOption[]>([])
const importForm = ref({ show: false, groupId: null as number | null, content: '' })
const importResult = ref<Record<string, number> | null>(null)

const openImport = async () => {
	importForm.value = { show: true, groupId: importForm.value.groupId, content: '' }
	importResult.value = null
	const res = await getGroupAll()
	if (isObject<{ list: { id: number; name: string }[] | null }>(res)) {
		groupOptions.value = (res.list ?? []).map(item => ({ label: item.name, value: item.id }))
	}
}

const handleFile = async (event: Event) => {
	const file = (event.target as HTMLInputElement).files?.[0]
	importForm.value.content = file ? await file.text() : ''
}

const handleImport = async () => {
	const res = await importLeads({ group_id: importForm.value.groupId!, content: importForm.value.content })
	if (isObject<Record<string, number>>(res)) {
		importResult.value = res
	}
	fetchTable()
}

// Add video
const videoForm = ref({ show: false, email: '', groupId: 0, videoUrl: '', thumbnailUrl: '', ctaUrl: '' })

const openVideo = (row: VideoStatus) => {
	videoForm.value = {
		show: true,
		email: row.contact_email,
		groupId: row.group_id,
		videoUrl: row.video_url || '',
		thumbnailUrl: row.thumbnail_url && row.thumbnail_url !== row.video_url ? row.thumbnail_url : '',
		ctaUrl: '',
	}
}

const handleSaveVideo = async () => {
	const { email, groupId, videoUrl, thumbnailUrl, ctaUrl } = videoForm.value
	await setVideo({
		contact_email: email,
		group_id: groupId,
		video_url: videoUrl.trim(),
		thumbnail_url: thumbnailUrl.trim() || undefined,
		cta_url: ctaUrl.trim() || undefined,
	})
	videoForm.value.show = false
	fetchTable()
}

const columns: DataTableColumns<VideoStatus> = [
	{
		title: 'Lead',
		key: 'contact_email',
		minWidth: 220,
		render: row => (
			<div>
				<div>{row.name || row.contact_email}</div>
				{row.name ? <div class="text-12px text-desc">{row.contact_email}</div> : null}
			</div>
		),
	},
	{
		title: 'Tier',
		key: 'lead_tier',
		width: 100,
		render: row => {
			const typeMap: Record<string, 'success' | 'info' | 'default'> = {
				tier_1: 'success',
				tier_2: 'info',
			}
			return (
				<NTag type={typeMap[row.lead_tier] || 'default'} size="small">
					{row.lead_tier || '-'}
				</NTag>
			)
		},
	},
	{
		title: 'Score',
		key: 'lead_score',
		width: 80,
	},
	{
		title: 'Why',
		key: 'lead_reasons',
		minWidth: 260,
		ellipsis: { tooltip: true },
		render: row => row.lead_reasons || '-',
	},
	{
		title: 'Status',
		key: 'status',
		width: 120,
		render: row => {
			const typeMap: Record<string, 'success' | 'warning' | 'default'> = {
				ready: 'success',
				pending: 'warning',
			}
			return (
				<NTag type={typeMap[row.status] || 'default'} size="small">
					{row.status}
				</NTag>
			)
		},
	},
	{
		title: 'Opens',
		key: 'views',
		width: 70,
		render: row => row.views || '-',
	},
	{
		title: 'Plays',
		key: 'plays',
		width: 70,
		render: row => row.plays || '-',
	},
	{
		title: 'Calls',
		key: 'clicks',
		width: 80,
		render: row =>
			row.clicks ? (
				<NTag type="success" size="small">
					{row.clicks}
				</NTag>
			) : (
				'-'
			),
	},
	{
		title: 'Last seen',
		key: 'last_activity',
		width: 160,
		render: row => (row.last_activity ? formatTime(row.last_activity) : '-'),
	},
	{
		title: 'Video',
		key: 'video_url',
		width: 160,
		render: row => (
			<div class="flex gap-12px">
				{row.landing_page_url ? (
					<NButton text type="primary" onClick={() => openLanding(row.landing_page_url)}>
						View
					</NButton>
				) : null}
				<NButton text type="primary" onClick={() => openVideo(row)}>
					{row.video_url ? 'Edit video' : 'Add video'}
				</NButton>
			</div>
		),
	},
]
</script>
