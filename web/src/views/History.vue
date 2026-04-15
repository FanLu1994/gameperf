<template>
  <div class="history">
    <el-card>
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>📚 历史采集记录</span>
          <el-button type="primary" @click="$router.push('/compare')">前往对比分析</el-button>
        </div>
      </template>
      <el-table :data="sessions" stripe v-loading="loading">
        <el-table-column prop="name" label="标签" width="200">
          <template #default="{ row }">
            <strong>{{ row.name || '-' }}</strong>
          </template>
        </el-table-column>
        <el-table-column prop="process" label="进程" width="150" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'running' ? 'success' : row.status === 'created' ? 'warning' : 'info'" size="small">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="start_time" label="开始时间" width="220" />
        <el-table-column prop="end_time" label="结束时间" width="220">
          <template #default="{ row }">{{ row.end_time || '-' }}</template>
        </el-table-column>
        <el-table-column label="统计摘要">
          <template #default="{ row }">
            <div v-if="summaries[row.id]" class="summary-row">
              <span>CPU {{ summaries[row.id].avg_cpu?.toFixed(1) }}% / 内存 {{ summaries[row.id].avg_memory?.toFixed(0) }}MB</span>
              <span v-if="summaries[row.id].avg_fps"> / FPS {{ summaries[row.id].avg_fps?.toFixed(1) }}</span>
            </div>
            <span v-else class="no-data">-</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="showDetail(row.id)">查看详情</el-button>
            <el-button size="small" type="primary" @click="$router.push('/compare?highlight=' + row.id)">对比</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 详情弹窗 -->
    <el-dialog v-model="detailVisible" :title="detailData?.session?.name || '采集详情'" width="900px">
      <div v-if="detailData" class="detail-content">
        <el-descriptions :column="3" border>
          <el-descriptions-item label="采样数">{{ detailData.sample_count }}</el-descriptions-item>
          <el-descriptions-item label="时长">{{ detailData.duration?.toFixed(0) }}秒</el-descriptions-item>
          <el-descriptions-item label="平均CPU">{{ detailData.avg_cpu?.toFixed(1) }}%</el-descriptions-item>
          <el-descriptions-item label="最大CPU">{{ detailData.max_cpu?.toFixed(1) }}%</el-descriptions-item>
          <el-descriptions-item label="P95 CPU">{{ detailData.p95_cpu?.toFixed(1) }}%</el-descriptions-item>
          <el-descriptions-item label="平均内存">{{ detailData.avg_memory?.toFixed(1) }}MB</el-descriptions-item>
          <el-descriptions-item label="最大内存">{{ detailData.max_memory?.toFixed(1) }}MB</el-descriptions-item>
          <el-descriptions-item label="P95 内存">{{ detailData.p95_memory?.toFixed(1) }}MB</el-descriptions-item>
          <el-descriptions-item v-if="detailData.avg_fps" label="平均FPS">{{ detailData.avg_fps?.toFixed(1) }}</el-descriptions-item>
          <el-descriptions-item v-if="detailData.min_fps" label="最低FPS">{{ detailData.min_fps?.toFixed(1) }}</el-descriptions-item>
        </el-descriptions>
        <div ref="detailChart" class="detail-chart"></div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import { listSessions, getSummary, getSamples, deleteSession } from '../api'
import { ElMessage } from 'element-plus'

const sessions = ref([])
const summaries = ref({})
const loading = ref(false)
const detailVisible = ref(false)
const detailData = ref(null)
const detailChart = ref(null)
let detailInstance = null

async function loadSessions() {
  loading.value = true
  try {
    const { data } = await listSessions()
    sessions.value = data
    // 并行加载摘要
    for (const s of data.slice(0, 20)) {
      getSummary(s.id).then(({ data }) => {
        summaries.value[s.id] = data
      }).catch(() => {})
    }
  } catch (e) { console.error(e) }
  loading.value = false
}

async function showDetail(id) {
  try {
    const { data: summary } = await getSummary(id)
    const { data: samples } = await getSamples(id)
    detailData.value = { ...summary, samples }
    detailVisible.value = true

    await nextTick()
    if (!detailInstance) {
      detailInstance = echarts.init(detailChart.value, 'dark')
    }

    const times = samples.map(s => {
      const m = Math.floor(s.elapsed / 60)
      const sec = Math.floor(s.elapsed % 60)
      return `${m}:${sec.toString().padStart(2, '0')}`
    })

    detailInstance.setOption({
      backgroundColor: 'transparent',
      tooltip: { trigger: 'axis' },
      legend: { data: ['CPU%', '内存MB', '线程数'], textStyle: { color: '#999' } },
      grid: { top: 40, right: 60, bottom: 30, left: 60 },
      xAxis: { type: 'category', data: times, axisLabel: { color: '#666' } },
      yAxis: [
        { type: 'value', axisLabel: { color: '#666' }, splitLine: { lineStyle: { color: '#1a1a3a' } } },
        { type: 'value', axisLabel: { color: '#666' }, splitLine: { show: false } },
      ],
      series: [
        { name: 'CPU%', type: 'line', smooth: true, showSymbol: false, data: samples.map(s => s.cpu.toFixed(1)), itemStyle: { color: '#00d4ff' } },
        { name: '内存MB', type: 'line', smooth: true, showSymbol: false, data: samples.map(s => s.memory.toFixed(1)), itemStyle: { color: '#36d399' } },
        { name: '线程数', type: 'line', smooth: true, showSymbol: false, yAxisIndex: 1, data: samples.map(s => s.threads), itemStyle: { color: '#f59e0b' } },
      ]
    })
  } catch (e) {
    ElMessage.error('加载失败')
  }
}

async function handleDelete(id) {
  try {
    await deleteSession(id)
    ElMessage.success('已删除')
    loadSessions()
  } catch (e) {
    ElMessage.error('删除失败')
  }
}

onMounted(loadSessions)
</script>

<style scoped>
.summary-row { font-size: 12px; color: #aaa; }
.no-data { color: #555; }
.detail-chart { height: 350px; margin-top: 20px; }
</style>
