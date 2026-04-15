<template>
  <div class="compare">
    <el-card>
      <template #header><span>⚖️ 对比分析</span></template>
      <div class="selector">
        <el-select v-model="selectedIds" multiple placeholder="选择 2+ 条记录" style="width:700px">
          <el-option v-for="s in sessions" :key="s.id" :label="(s.name||s.id)+' - '+s.start_time" :value="s.id" />
        </el-select>
        <el-button type="primary" @click="loadCompare" :loading="loading" :disabled="selectedIds.length<2">
          对比 ({{ selectedIds.length }})
        </el-button>
      </div>
    </el-card>

    <!-- 摘要表 -->
    <el-card v-if="results.length" style="margin-top:16px">
      <template #header><span>📋 对比摘要</span></template>
      <el-table :data="results" stripe>
        <el-table-column label="标签" width="160">
          <template #default="{ row }"><strong>{{ row.session?.name || row.session?.id }}</strong></template>
        </el-table-column>
        <el-table-column label="时长/采样" width="120">
          <template #default="{ row }">{{ row.duration?.toFixed(0) }}s / {{ row.sample_count }}</template>
        </el-table-column>
        <el-table-column label="CPU Avg/Max/P95" width="200">
          <template #default="{ row }">
            <span :class="{ 'metric-danger': row.avg_cpu > 80, 'metric-warning': row.avg_cpu > 50 }">
              {{ row.avg_cpu?.toFixed(1) }} / {{ row.max_cpu?.toFixed(1) }} / {{ row.p95_cpu?.toFixed(1) }} %
            </span>
          </template>
        </el-table-column>
        <el-table-column label="内存 Max/P95" width="160">
          <template #default="{ row }">{{ row.max_memory?.toFixed(0) }} / {{ row.p95_memory?.toFixed(0) }} MB</template>
        </el-table-column>
        <el-table-column label="GPU Avg/Max" width="140">
          <template #default="{ row }">{{ row.avg_gpu?.toFixed(1) || '-' }} / {{ row.max_gpu?.toFixed(1) || '-' }} %</template>
        </el-table-column>
        <el-table-column label="FPS Avg/Min/P1" width="180">
          <template #default="{ row }">
            <span v-if="row.avg_fps">{{ row.avg_fps?.toFixed(1) }} / {{ row.min_fps?.toFixed(1) }} / {{ row.p1_fps?.toFixed(1) }}</span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="帧时间 P95/P99" width="160">
          <template #default="{ row }">
            <span v-if="row.avg_frame_time">{{ row.p95_frame_time?.toFixed(2) }} / {{ row.p99_frame_time?.toFixed(2) }} ms</span>
          </template>
        </el-table-column>
        <el-table-column label="卡顿" width="80">
          <template #default="{ row }">{{ row.total_jank_count || 0 }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 对比图表 -->
    <div v-if="results.length" class="compare-charts">
      <el-card><template #header><span>CPU 对比</span></template><div ref="cpuCmp" class="chart-lg"></div></el-card>
      <el-card><template #header><span>内存 对比</span></template><div ref="memCmp" class="chart-lg"></div></el-card>
      <el-card v-if="results.some(r => r.avg_gpu)"><template #header><span>GPU 对比</span></template><div ref="gpuCmp" class="chart-lg"></div></el-card>
      <el-card v-if="results.some(r => r.avg_fps)"><template #header><span>FPS 对比</span></template><div ref="fpsCmp" class="chart-lg"></div></el-card>
      <el-card><template #header><span>磁盘 I/O 对比</span></template><div ref="diskCmp" class="chart-lg"></div></el-card>
      <el-card><template #header><span>网络 I/O 对比</span></template><div ref="netCmp" class="chart-lg"></div></el-card>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import * as echarts from 'echarts'
import { listSessions, getSamples } from '../api'
import { ElMessage } from 'element-plus'

const route = useRoute()
const sessions = ref([])
const selectedIds = ref([])
const results = ref([])
const loading = ref(false)

const cpuCmp = ref(null), memCmp = ref(null), gpuCmp = ref(null), fpsCmp = ref(null), diskCmp = ref(null), netCmp = ref(null)
let charts = {}

const colors = ['#00d4ff', '#36d399', '#f59e0b', '#ef4444', '#a855f7', '#ec4899']

async function loadCompare() {
  if (selectedIds.value.length < 2) return
  loading.value = true
  try {
    const allData = []
    for (const id of selectedIds.value) {
      const { data } = await getSamples(id)
      const session = sessions.value.find(s => s.id === id)
      allData.push({ id, session, samples: data })
    }

    results.value = allData.map(d => {
      const s = d.samples
      return {
        session: d.session, sample_count: s.length,
        duration: s.length > 0 ? s[s.length - 1].elapsed : 0,
        avg_cpu: avg(s.map(x => x.cpu)), max_cpu: max(s.map(x => x.cpu)), p95_cpu: p95(s.map(x => x.cpu)),
        max_memory: max(s.map(x => x.memory)), p95_memory: p95(s.map(x => x.memory)),
        avg_gpu: avg(s.filter(x => x.gpu_util > 0).map(x => x.gpu_util)),
        max_gpu: max(s.filter(x => x.gpu_util > 0).map(x => x.gpu_util)),
        avg_fps: avg(s.filter(x => x.fps > 0).map(x => x.fps)),
        min_fps: min(s.filter(x => x.fps > 0).map(x => x.fps)),
        p1_fps: p1(s.filter(x => x.fps > 0).map(x => x.fps)),
        avg_frame_time: avg(s.filter(x => x.frame_time > 0).map(x => x.frame_time)),
        p95_frame_time: p95(s.filter(x => x.frame_time > 0).map(x => x.frame_time)),
        p99_frame_time: p99(s.filter(x => x.frame_time > 0).map(x => x.frame_time)),
        total_jank_count: s.reduce((a, x) => a + (x.jank_count || 0), 0),
      }
    })

    await nextTick()
    drawCompare(allData)
  } catch { ElMessage.error('对比失败') }
  loading.value = false
}

function drawCompare(allData) {
  const targets = [
    { ref: cpuCmp, key: 'cpu', name: 'CPU%', unit: '%' },
    { ref: memCmp, key: 'memory', name: '内存', unit: 'MB' },
    { ref: gpuCmp, key: 'gpu_util', name: 'GPU%', unit: '%' },
    { ref: fpsCmp, key: 'fps', name: 'FPS', unit: '' },
    { ref: diskCmp, key: 'disk_read_bps', key2: 'disk_write_bps', name: '磁盘', unit: '' },
    { ref: netCmp, key: 'net_sent_bps', key2: 'net_recv_bps', name: '网络', unit: '' },
  ]

  targets.forEach((t, i) => {
    const el = t.ref.value
    if (!el) return
    if (!charts[t.key]) charts[t.key] = echarts.init(el, 'dark')

    const seriesList = []
    allData.forEach((d, j) => {
      const name = d.session?.name || d.id
      if (t.key2) {
        seriesList.push({ name: name + ' 读', type: 'line', smooth: true, showSymbol: false, data: d.samples.map(s => [s.elapsed, s[t.key]]), itemStyle: { color: colors[j] } })
        seriesList.push({ name: name + ' 写', type: 'line', smooth: true, showSymbol: false, lineStyle: { type: 'dashed' }, data: d.samples.map(s => [s.elapsed, s[t.key2]]), itemStyle: { color: colors[j] } })
      } else {
        seriesList.push({ name, type: 'line', smooth: true, showSymbol: false, data: d.samples.map(s => [s.elapsed, s[t.key]]), itemStyle: { color: colors[j % colors.length] } })
      }
    })

    charts[t.key].setOption({
      backgroundColor: 'transparent', tooltip: { trigger: 'axis' },
      legend: { textStyle: { color: '#999' } },
      grid: { top: 40, right: 20, bottom: 30, left: 60 },
      xAxis: { type: 'value', name: '秒', axisLabel: { color: '#666' } },
      yAxis: { type: 'value', axisLabel: { color: '#666' }, splitLine: { lineStyle: { color: '#1a1a3a' } } },
      series: seriesList,
    })
  })
}

function avg(arr) { return arr.length ? arr.reduce((a, b) => a + b, 0) / arr.length : 0 }
function max(arr) { return arr.length ? Math.max(...arr) : 0 }
function min(arr) { return arr.length ? Math.min(...arr) : 0 }
function p1(arr) { if (!arr.length) return 0; const s = [...arr].sort((a,b)=>a-b); return s[Math.floor(s.length*0.01)] }
function p95(arr) { if (!arr.length) return 0; const s = [...arr].sort((a,b)=>a-b); return s[Math.floor(s.length*0.95)] }
function p99(arr) { if (!arr.length) return 0; const s = [...arr].sort((a,b)=>a-b); return s[Math.floor(s.length*0.99)] }

onMounted(async () => {
  const { data } = await listSessions()
  sessions.value = data
  if (route.query.highlight) selectedIds.value = [route.query.highlight]
})
</script>

<style scoped>
.selector { display: flex; gap: 12px; align-items: center; }
.compare-charts { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-top: 16px; }
.chart-lg { height: 300px; }
.metric-danger { color: #ef4444; font-weight: 700; }
.metric-warning { color: #f59e0b; }
</style>
