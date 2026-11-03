<template>
	<bt-table-layout>
		<template #toolsLeft>
			<div class="w-200px">
				<n-select
					v-model:value="tableParams.status"
					:options="statusOptions"
					@update:value="() => resetTable()">
				</n-select>
			</div>
			<bt-search
				v-model:value="tableParams.keyword"
				placeholder="Search recipient or sender"
				@search="() => resetTable()">
			</bt-search>
		</template>
		<template #table>
			<n-data-table v-bind="tableProps" :columns="columns"></n-data-table>
		</template>
		<template #pageRight>
			<bt-table-page v-bind="pageProps" @refresh="fetchTable"> </bt-table-page>
		</template>
	</bt-table-layout>
</template>

<script lang="tsx" setup>
import { DataTableColumns, NTag } from 'naive-ui'
import { formatTime } from '@/utils'
import { useDataTable } from '@/hooks/useDataTable'
import { getDeliveryLogs, type DeliveryLog } from '@/api/modules/delivery-log'

const statusOptions = [
	{ label: 'All statuses', value: '' },
	{ label: 'Sent', value: 'sent' },
	{ label: 'Bounced', value: 'bounced' },
	{ label: 'Deferred', value: 'deferred' },
	{ label: 'Expired', value: 'expired' },
]

const statusType: Record<string, 'success' | 'error' | 'warning' | 'default'> = {
	sent: 'success',
	bounced: 'error',
	expired: 'error',
	deferred: 'warning',
}

const columns: DataTableColumns<DeliveryLog> = [
	{
		key: 'log_time',
		title: 'Time',
		width: 160,
		render: row => formatTime(row.log_time),
	},
	{
		key: 'status',
		title: 'Status',
		width: 100,
		render: row => (
			<NTag type={statusType[row.status] || 'default'} size="small">
				{row.status}
			</NTag>
		),
	},
	{ key: 'sender', title: 'From', width: 200, ellipsis: { tooltip: true } },
	{ key: 'recipient', title: 'To', width: 200, ellipsis: { tooltip: true } },
	{ key: 'dsn', title: 'Code', width: 70 },
	{ key: 'reason', title: 'Reason', minWidth: 220, ellipsis: { tooltip: true } },
	{ key: 'description', title: 'Server response', minWidth: 260, ellipsis: { tooltip: true } },
	{ key: 'relay', title: 'Via', width: 180, ellipsis: { tooltip: true } },
]

const { tableParams, tableProps, pageProps, fetchTable, resetTable } = useDataTable<DeliveryLog>({
	params: {
		status: '',
		keyword: '',
		page: 1,
		page_size: 20,
	},
	immediate: true,
	rowKey: row => row.postfix_message_id,
	fetchFn: getDeliveryLogs,
})
</script>
