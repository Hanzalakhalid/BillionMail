<template>
	<div class="p-24px">
		<n-tabs v-model:value="activeTab" type="line" animated>
			<n-tab-pane name="find" tab="Find advertisers">
				<n-alert v-if="!config.apify_token" type="warning" class="mb-16px">
					Add your Apify API token in the Settings tab first.
				</n-alert>

				<n-card size="small" class="mb-16px">
					<div class="flex flex-wrap items-center gap-12px">
						<n-input
							v-model:value="search.keyword"
							placeholder="What do they advertise? e.g. dental Karachi, cafe Riyadh"
							style="width: 360px"
							@keyup.enter="handleSearch" />
						<n-select v-model:value="search.country" :options="countryOptions" style="width: 170px" />
						<n-input-number v-model:value="search.limit" :min="10" :max="500" :step="10" style="width: 140px">
							<template #suffix>ads</template>
						</n-input-number>
						<n-button type="primary" :disabled="!config.apify_token || !search.keyword.trim()" @click="handleSearch">
							Find ads
						</n-button>
					</div>
					<div class="mt-8px text-12px text-desc">
						Apify charges per ad fetched (about $5.80 per 1,000 ads on the free plan). Fewer ads = cheaper.
					</div>
				</n-card>

				<bt-table-layout>
					<template #toolsLeft>
						<n-select
							v-model:value="tableParams.keyword"
							:options="keywordOptions"
							placeholder="All searches"
							clearable
							style="width: 240px"
							@update:value="() => fetchTable(true)" />
					</template>
					<template #toolsRight>
						<bt-search
							v-model:value="tableParams.search"
							placeholder="Search name, website or ad"
							@search="() => fetchTable(true)" />
					</template>
					<template #table>
						<n-data-table v-bind="tableProps" :columns="columns" :row-key="(row: Advertiser) => row.page_id">
							<template #empty>
								<n-empty description="No advertisers yet. Search above to find businesses running ads." />
							</template>
						</n-data-table>
					</template>
					<template #pageRight>
						<bt-table-page v-bind="pageProps" @refresh="fetchTable"> </bt-table-page>
					</template>
				</bt-table-layout>
			</n-tab-pane>

			<n-tab-pane name="settings" tab="Settings">
				<n-alert type="info" class="mb-16px max-w-640px">
					Uses the "Facebook Ads Library Scraper" on Apify. Copy your API token from Apify Console > Settings >
					API & Integrations.
				</n-alert>
				<n-form label-placement="top" class="max-w-640px">
					<n-form-item label="Apify API token">
						<n-input
							v-model:value="config.apify_token"
							type="password"
							show-password-on="click"
							:input-props="{ autocomplete: 'new-password' }" />
					</n-form-item>
					<n-form-item label="Default country">
						<n-select v-model:value="config.default_country" :options="countryOptions" />
					</n-form-item>
					<n-form-item label="Ads per search" feedback="You pay per ad. 50-100 is enough to find the main advertisers.">
						<n-input-number v-model:value="config.results_limit" :min="10" :max="500" :step="10" />
					</n-form-item>
					<n-button type="primary" @click="handleSaveConfig">Save</n-button>
				</n-form>
			</n-tab-pane>
		</n-tabs>

		<n-modal v-model:show="leadForm.show" preset="card" title="Add to leads" :style="{ width: '520px' }">
			<n-form label-placement="top">
				<n-form-item label="Business">
					<n-input :value="leadForm.name" disabled />
				</n-form-item>
				<n-form-item label="Contact group">
					<n-select v-model:value="leadForm.groupId" :options="groupOptions" placeholder="Select a group" filterable />
				</n-form-item>
				<n-form-item
					label="Email (optional)"
					feedback="Leave empty to find it on their website. If none is found, copy one from their Facebook page.">
					<n-input v-model:value="leadForm.email" placeholder="owner@business.com" />
				</n-form-item>
			</n-form>
			<n-button type="primary" :disabled="!leadForm.groupId" @click="handleAddLead">Add to leads</n-button>
		</n-modal>
	</div>
</template>

<script lang="tsx" setup>
import { DataTableColumns, NButton, NTag, NTooltip, SelectOption } from 'naive-ui'
import { useDataTable } from '@/hooks/useDataTable'
import { isObject } from '@/utils'
import { getGroupAll } from '@/api/modules/contacts/group'
import {
	getAdFinderConfig,
	saveAdFinderConfig,
	searchAds,
	listAdvertisers,
	addAdvertiserLead,
	type AdFinderConfig,
	type Advertiser,
} from '@/api/modules/ad-finder'

const activeTab = ref('find')

const countryOptions = [
	{ label: 'Pakistan', value: 'PK' },
	{ label: 'Saudi Arabia', value: 'SA' },
	{ label: 'UAE', value: 'AE' },
	{ label: 'Qatar', value: 'QA' },
	{ label: 'United Kingdom', value: 'GB' },
	{ label: 'United States', value: 'US' },
	{ label: 'All countries', value: 'ALL' },
]

// Settings
const config = ref<AdFinderConfig>({ apify_token: '', default_country: 'PK', results_limit: 100 })

const loadConfig = async () => {
	const res = await getAdFinderConfig()
	if (isObject<AdFinderConfig>(res)) {
		config.value = { ...config.value, ...res }
		search.value.country = config.value.default_country
		search.value.limit = config.value.results_limit
	}
}

const handleSaveConfig = async () => {
	await saveAdFinderConfig(config.value)
	await loadConfig()
}

// Search
const search = ref({ keyword: '', country: 'PK', limit: 100 })

const handleSearch = async () => {
	const keyword = search.value.keyword.trim()
	if (!keyword || !config.value.apify_token) return
	await searchAds({ keyword, country: search.value.country, limit: search.value.limit })
	tableParams.value.keyword = keyword
	fetchTable(true)
}

// Results
const keywordOptions = ref<SelectOption[]>([])

const listWithKeywords = async (params: any) => {
	const res = await listAdvertisers(params)
	if (isObject<{ keywords: string[] | null }>(res)) {
		keywordOptions.value = (res.keywords ?? []).filter(Boolean).map(k => ({ label: k, value: k }))
	}
	return res
}

const { tableProps, pageProps, tableParams, fetchTable } = useDataTable<Advertiser>({
	params: {
		page: 1,
		page_size: 20,
		keyword: null as string | null,
		search: '',
	},
	rowKey: row => row.page_id,
	fetchFn: listWithKeywords,
})

// Add to leads
const groupOptions = ref<SelectOption[]>([])
const leadForm = ref({ show: false, pageId: '', name: '', groupId: null as number | null, email: '' })

const openAddLead = async (row: Advertiser) => {
	leadForm.value = { show: true, pageId: row.page_id, name: row.page_name, groupId: leadForm.value.groupId, email: '' }
	const res = await getGroupAll()
	if (isObject<{ list: { id: number; name: string }[] | null }>(res)) {
		groupOptions.value = (res.list ?? []).map(item => ({ label: item.name, value: item.id }))
	}
}

const handleAddLead = async () => {
	const { pageId, groupId, email } = leadForm.value
	await addAdvertiserLead({ page_id: pageId, group_id: groupId!, email: email.trim() || undefined })
	leadForm.value.show = false
	fetchTable()
}

const scoreType = (score: number) => (score >= 75 ? 'success' : score >= 40 ? 'warning' : 'default')

const columns: DataTableColumns<Advertiser> = [
	{
		title: 'Business',
		key: 'page_name',
		minWidth: 200,
		render: row => (
			<div>
				<a href={row.page_url} target="_blank" rel="noopener" class="text-primary">
					{row.page_name}
				</a>
				<div class="text-12px text-desc">{row.category}</div>
				{row.website ? (
					<a href={row.website} target="_blank" rel="noopener" class="text-12px text-desc">
						{row.website.replace(/^https?:\/\//, '')}
					</a>
				) : null}
			</div>
		),
	},
	{
		title: 'What they advertise',
		key: 'ad_topic',
		minWidth: 260,
		render: row => (
			<NTooltip style={{ maxWidth: '420px' }}>
				{{
					trigger: () => <span class="line-clamp-2">{row.ad_topic || '-'}</span>,
					default: () => row.ad_text || row.ad_topic || '-',
				}}
			</NTooltip>
		),
	},
	{
		title: 'Running',
		key: 'days_running',
		width: 90,
		render: row => `${row.days_running} days`,
	},
	{
		title: 'Ads',
		key: 'ad_count',
		width: 60,
	},
	{
		title: 'Where',
		key: 'platforms',
		width: 140,
		render: row => (row.platforms ? row.platforms.split(',').join(', ').toLowerCase() : '-'),
	},
	{
		title: 'Budget score',
		key: 'score',
		width: 110,
		render: row => (
			<NTooltip style={{ maxWidth: '360px' }}>
				{{
					trigger: () => (
						<NTag type={scoreType(row.score)} size="small">
							{row.score}
						</NTag>
					),
					default: () => row.reasons,
				}}
			</NTooltip>
		),
	},
	{
		title: 'Actions',
		key: 'actions',
		width: 210,
		render: row => (
			<div class="flex gap-12px">
				<NButton text type="primary" tag="a" href={row.ad_library_url} target="_blank">
					See ads
				</NButton>
				{row.lead_email ? (
					<NTag type="success" size="small">
						Lead: {row.lead_email}
					</NTag>
				) : (
					<NButton text type="primary" onClick={() => openAddLead(row)}>
						Add to leads
					</NButton>
				)}
			</div>
		),
	},
]

onMounted(() => {
	loadConfig()
})
</script>
