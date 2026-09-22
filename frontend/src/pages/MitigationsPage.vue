<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { Check, Eye, RefreshCw, RotateCcw, SprayCan, X } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { mitigationApi, routeApi } from '@/api/domain'
import RiskBadge from '@/components/common/RiskBadge.vue'
import { useAuth } from '@/hooks/useAuth'
import type { ProcessRoute } from '@/types/domain'
import { measureTypeLabels, mitigationLabels, type MeasureType, type MitigationMeasure, type MitigationStatus } from '@/types/mitigation'
import { dateOnly, dateTime } from '@/utils/format'

const auth = useAuth()
const routes = ref<ProcessRoute[]>([])
const measures = ref<MitigationMeasure[]>([])
const total = ref(0)
const loading = ref(false)
const workingId = ref<number>()
const query = reactive({ route_id: undefined as number | undefined, status: '' as MitigationStatus | '' })
const detailOpen = ref(false)
const selected = ref<MitigationMeasure>()
const resubmitOpen = ref(false)
const reviewOpen = ref(false)
const reviewDecision = ref<'approved' | 'rejected'>('approved')
const reviewReason = ref('')
const resubmitForm = reactive({ measure_type: 'cleaning' as MeasureType, completed_on: '', evidence_note: '' })
const routesById = computed(() => Object.fromEntries(routes.value.map((route) => [route.id, route])))
const countBy = (status: MitigationStatus) => measures.value.filter((item) => item.mitigation_status === status).length
const statusLabel = (status: MitigationStatus) => mitigationLabels[status]

async function load() {
  loading.value = true
  try { const result = await mitigationApi.list({ route_id: query.route_id, status: query.status || undefined, page_size: 100 }); measures.value = result.items; total.value = result.total }
  finally { loading.value = false }
}
async function loadRoutes() { routes.value = (await routeApi.list({ page_size: 100 })).items }
function inspect(measure: MitigationMeasure) { selected.value = measure; detailOpen.value = true }
function openResubmit(measure: MitigationMeasure) { selected.value = measure; Object.assign(resubmitForm, { measure_type: measure.measure_type, completed_on: dateOnly(measure.completed_on), evidence_note: measure.evidence_note }); resubmitOpen.value = true }
async function resubmit() {
  if (!selected.value || !resubmitForm.completed_on || resubmitForm.evidence_note.trim().length < 4) return
  workingId.value = selected.value.id
  try { await mitigationApi.resubmit(selected.value.id, { ...resubmitForm }); ElMessage.success(`措施 #${selected.value.id} 已重新提交复核`); resubmitOpen.value = false; await load() }
  finally { workingId.value = undefined }
}
function openReview(measure: MitigationMeasure, decision: 'approved' | 'rejected') { selected.value = measure; reviewDecision.value = decision; reviewReason.value = ''; reviewOpen.value = true }
async function review() {
  if (!selected.value || reviewReason.value.trim().length < 4) return
  workingId.value = selected.value.id
  try { await mitigationApi.review(selected.value.id, reviewDecision.value, reviewReason.value); ElMessage.success(reviewDecision.value === 'approved' ? '措施已通过，矩阵将显示已缓解' : '措施已拒绝'); reviewOpen.value = false; await load() }
  finally { workingId.value = undefined }
}
onMounted(async () => { await Promise.all([loadRoutes(), load()]) })
</script>

<template>
  <header class="page-header"><div class="page-header-copy"><h1>缓解复核</h1><p>高风险路径的清洗 / 换线措施与复核追踪</p></div></header>
  <section class="content-band stack">
    <div class="metric-strip"><div class="metric"><span>待复核</span><strong>{{ countBy('pending_review') }}</strong></div><div class="metric"><span>待处理</span><strong>{{ countBy('pending') }}</strong></div><div class="metric"><span>已缓解</span><strong>{{ countBy('approved') }}</strong></div><div class="metric"><span>已拒绝</span><strong>{{ countBy('rejected') }}</strong></div></div>
    <div class="toolbar"><el-select v-model="query.route_id" clearable filterable placeholder="全部路线" style="width: 280px" @change="load"><el-option v-for="route in routes" :key="route.id" :label="`${route.route_code} · ${route.product_name}`" :value="route.id" /></el-select><el-select v-model="query.status" clearable placeholder="全部状态" style="width: 150px" @change="load"><el-option v-for="(label, status) in mitigationLabels" :key="status" :label="label" :value="status" /></el-select><el-tooltip content="刷新"><el-button :icon="RefreshCw" circle aria-label="刷新" @click="load" /></el-tooltip><span class="toolbar-spacer subtle-count">{{ total }} 条措施</span></div>
    <div class="data-surface">
      <div class="surface-header"><SprayCan :size="18" /><h2>缓解措施</h2><span>同一路径仅一条待复核记录</span></div>
      <el-table v-loading="loading" :data="measures" row-key="id">
        <el-table-column label="编号" width="80"><template #default="{ row }"><button class="link-button mono" @click="inspect(row)">#{{ row.id }}</button></template></el-table-column>
        <el-table-column label="路线 / 版本" min-width="165"><template #default="{ row }"><strong>{{ routesById[row.route_id]?.route_code || `路线 #${row.route_id}` }}</strong><div class="muted row-sub">登记于 v{{ row.route_version }}</div></template></el-table-column>
        <el-table-column label="过敏原" width="105"><template #default="{ row }"><span class="allergen-tag">{{ row.allergen }}</span></template></el-table-column>
        <el-table-column label="路径" min-width="165"><template #default="{ row }"><span class="mono">{{ row.source_step_code }} → {{ row.target_step_code }}</span></template></el-table-column>
        <el-table-column label="措施 / 完成日期" min-width="150"><template #default="{ row }">{{ measureTypeLabels[row.measure_type as MeasureType] }}<div class="muted row-sub">{{ dateOnly(row.completed_on) }}</div></template></el-table-column>
        <el-table-column label="原始风险" width="140"><template #default="{ row }"><RiskBadge :level="row.risk_level" :score="row.raw_score" /></template></el-table-column>
        <el-table-column label="状态" width="115"><template #default="{ row }"><span class="status-pill" :class="`mitigation-${row.mitigation_status}`">{{ statusLabel(row.mitigation_status) }}</span></template></el-table-column>
        <el-table-column label="操作" min-width="190" fixed="right"><template #default="{ row }"><el-button :icon="Eye" text @click="inspect(row)">查看</el-button><el-button v-if="auth.canEdit.value && row.mitigation_status === 'pending'" type="primary" text :icon="RotateCcw" @click="openResubmit(row)">重新提交</el-button><template v-if="auth.canReview.value && row.mitigation_status === 'pending_review'"><el-button type="success" text :icon="Check" @click="openReview(row, 'approved')">通过</el-button><el-button type="danger" text :icon="X" @click="openReview(row, 'rejected')">拒绝</el-button></template></template></el-table-column>
        <template #empty><div class="empty-state"><div><strong>暂无缓解措施</strong><span>在交叉接触矩阵的高风险路径上登记清洗或换线措施</span></div></div></template>
      </el-table>
    </div>
  </section>

  <el-drawer v-model="detailOpen" title="措施详情" size="min(620px, 94vw)">
    <template v-if="selected">
      <div class="measure-head"><div><span>措施 #{{ selected.id }}</span><strong>{{ selected.allergen }} · {{ selected.source_step_code }} → {{ selected.target_step_code }}</strong></div><span class="status-pill" :class="`mitigation-${selected.mitigation_status}`">{{ statusLabel(selected.mitigation_status) }}</span><RiskBadge :level="selected.risk_level" :score="selected.raw_score" /></div>
      <dl class="measure-facts"><div><dt>路线版本</dt><dd>v{{ selected.route_version }}</dd></div><div><dt>措施类型</dt><dd>{{ measureTypeLabels[selected.measure_type as MeasureType] }}</dd></div><div><dt>完成日期</dt><dd>{{ dateOnly(selected.completed_on) }}</dd></div><div><dt>提交时间</dt><dd>{{ dateTime(selected.created_at) }}</dd></div><div><dt>复核时间</dt><dd>{{ dateTime(selected.reviewed_at) }}</dd></div><div><dt>复核人</dt><dd>{{ selected.reviewed_by ? `#${selected.reviewed_by}` : '—' }}</dd></div></dl>
      <section class="note-block"><span>完成证据</span><p>{{ selected.evidence_note }}</p></section>
      <section v-if="selected.review_reason" class="note-block review"><span>复核理由</span><p>{{ selected.review_reason }}</p></section>
    </template>
  </el-drawer>

  <el-dialog v-model="resubmitOpen" title="重新提交缓解措施" width="540px">
    <div v-if="selected" class="review-target">{{ selected.allergen }} · {{ selected.source_step_code }} → {{ selected.target_step_code }}</div>
    <el-form label-position="top">
      <el-form-item label="措施类型" required><el-radio-group v-model="resubmitForm.measure_type"><el-radio-button value="cleaning">清洗</el-radio-button><el-radio-button value="line_change">换线</el-radio-button></el-radio-group></el-form-item>
      <el-form-item label="完成日期（不晚于提交日）" required><el-date-picker v-model="resubmitForm.completed_on" type="date" value-format="YYYY-MM-DD" placeholder="选择完成日期" :disabled-date="(day: Date) => day.getTime() > Date.now()" style="width: 100%" /></el-form-item>
      <el-form-item label="完成证据" required><el-input v-model="resubmitForm.evidence_note" type="textarea" :rows="4" maxlength="1000" show-word-limit placeholder="清洗或换线记录、检查单与附件位置" /></el-form-item>
    </el-form>
    <template #footer><el-button @click="resubmitOpen = false">取消</el-button><el-button type="primary" :loading="workingId === selected?.id" :disabled="!resubmitForm.completed_on || resubmitForm.evidence_note.trim().length < 4" @click="resubmit">提交复核</el-button></template>
  </el-dialog>

  <el-dialog v-model="reviewOpen" :title="reviewDecision === 'approved' ? '通过缓解措施' : '拒绝缓解措施'" width="540px">
    <div v-if="selected" class="review-target">措施 #{{ selected.id }} · {{ selected.allergen }} · {{ selected.source_step_code }} → {{ selected.target_step_code }}</div>
    <el-form label-position="top"><el-form-item label="复核理由" required><el-input v-model="reviewReason" type="textarea" :rows="4" maxlength="1000" show-word-limit placeholder="记录证据判断与后续处置依据" /></el-form-item></el-form>
    <template #footer><el-button @click="reviewOpen = false">取消</el-button><el-button :type="reviewDecision === 'approved' ? 'success' : 'danger'" :loading="workingId === selected?.id" :disabled="reviewReason.trim().length < 4" @click="review">确认{{ reviewDecision === 'approved' ? '通过' : '拒绝' }}</el-button></template>
  </el-dialog>
</template>

<style scoped>
.row-sub { margin-top: 3px; font-size: 11px; }.measure-head { display: flex; align-items: center; gap: 10px; padding: 12px; background: var(--surface-muted); border: 1px solid var(--line); }.measure-head > div { margin-right: auto; }.measure-head div span, .measure-head div strong { display: block; }.measure-head div span { color: var(--muted); font-size: 10px; }.measure-head div strong { margin-top: 3px; font-size: 13px; }.measure-facts { display: grid; grid-template-columns: repeat(3, 1fr); margin: 12px 0; border: 1px solid var(--line); }.measure-facts div { padding: 9px 11px; border-right: 1px solid var(--line); border-bottom: 1px solid var(--line); }.measure-facts div:nth-child(3n) { border-right: 0; }.measure-facts div:nth-child(n+4) { border-bottom: 0; }.measure-facts dt { color: var(--muted); font-size: 10px; }.measure-facts dd { margin: 4px 0 0; font-size: 11px; font-weight: 700; }.note-block { margin-top: 12px; border: 1px solid var(--line); }.note-block > span { display: block; padding: 6px 10px; color: #28604d; background: #e9f3ee; border-bottom: 1px solid var(--line); font-size: 11px; font-weight: 800; }.note-block.review > span { color: #78540f; background: #fff4d8; }.note-block p { margin: 0; padding: 12px; color: #4c5752; font-size: 12px; line-height: 1.55; white-space: pre-wrap; }.review-target { margin-bottom: 16px; padding: 10px 12px; background: var(--surface-muted); border: 1px solid var(--line); font: 12px "SFMono-Regular", Consolas, monospace; }
@media (max-width: 600px) { .measure-facts { grid-template-columns: 1fr; }.measure-facts div { border-right: 0; border-bottom: 1px solid var(--line); }.measure-facts div:last-child { border-bottom: 0; } }
</style>
