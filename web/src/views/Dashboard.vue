<template>
  <div class="dashboard">
    <!-- 平台选择 -->
    <el-card class="control-card">
      <template #header>
        <div style="display:flex;align-items:center;gap:12px">
          <span>🚀 新建采集</span>
          <el-radio-group v-model="platform" size="small">
            <el-radio-button value="windows">🖥️ Windows/Linux</el-radio-button>
            <el-radio-button value="android">📱 Android</el-radio-button>
            <el-radio-button value="ios">🍎 iOS</el-radio-button>
          </el-radio-group>
        </div>
      </template>

      <!-- Windows/Linux -->
      <el-form v-if="platform === 'windows'" :inline="true" :model="form" label-width="80px">
        <el-form-item label="标签"><el-input v-model="form.name" placeholder="v1.0优化前" style="width:160px" /></el-form-item>
        <el-form-item label="PID"><el-input-number v-model="form.pid" :min="1" /></el-form-item>
        <el-form-item label="间隔">
          <el-select v-model="form.interval" style="width:110px">
            <el-option label="500ms" value="500ms" /><el-option label="1秒" value="1s" /><el-option label="2秒" value="2s" />
          </el-select>
        </el-form-item>
        <el-form-item><el-button type="primary" @click="handleCreate" :loading="creating">开始</el-button></el-form-item>
      </el-form>

      <!-- Android -->
      <div v-else-if="platform === 'android'">
        <el-form :inline="true" :model="androidForm" label-width="80px">
          <el-form-item label="设备">
            <el-select v-model="androidForm.device_id" placeholder="选择设备" style="width:200px" @focus="loadDevices">
              <el-option v-for="d in devices" :key="d" :label="d" :value="d" />
            </el-select>
          </el-form-item>
          <el-form-item label="包名">
            <el-select v-model="androidForm.package" filterable placeholder="选择或输入包名" style="width:300px" @focus="loadPackages">
              <el-option v-for="p in packages" :key="p" :label="p" :value="p" />
            </el-select>
          </el-form-item>
          <el-form-item label="标签"><el-input v-model="androidForm.name" placeholder="v1.0优化前" style="width:140px" /></el-form-item>
          <el-form-item label="间隔">
            <el-select v-model="androidForm.interval" style="width:110px">
              <el-option label="1秒" value="1s" /><el-option label="2秒" value="2s" /><el-option label="5秒" value="5s" />
            </el-select>
          </el-form-item>
          <el-form-item><el-button type="primary" @click="handleAndroidCreate" :loading="creating">开始</el-button></el-form-item>
        </el-form>
      </div>

      <!-- iOS -->
      <div v-else-if="platform === 'ios'">
        <el-alert v-if="!iosReady" type="warning" :closable="false" style="margin-bottom:12px">
          iOS 采集需要 macOS + pymobiledevice3 (pip3 install pymobiledevice3)
        </el-alert>
        <el-form :inline="true" :model="iosForm" label-width="80px">
          <el-form-item label="设备">
            <el-select v-model="iosForm.device_id" placeholder="选择 iOS 设备" style="width:220px" @focus="loadIOSDevices">
              <el-option v-for="d in iosDevices" :key="d.udid" :label="d.name + ' (' + d.version + ')'" :value="d.udid" />
            </el-select>
          </el-form-item>
          <el-form-item label="Bundle ID">
            <el-select v-model="iosForm.package" filterable allow-create placeholder="com.example.app" style="width:300px" @focus="loadIOSApps">
              <el-option v-for="a in iosApps" :key="a" :label="a" :value="a.split(' (')[0].trim()" />
            </el-select>
          </el-form-item>
          <el-form-item label="标签"><el-input v-model="iosForm.name" placeholder="v1.0优化前" style="width:140px" /></el-form-item>
          <el-form-item label="间隔">
            <el-select v-model="iosForm.interval" style="width:110px">
              <el-option label="2秒" value="2s" /><el-option label="5秒" value="5s" /><el-option label="10秒" value="10s" />
            </el-select>
          </el-form-item>
          <el-form-item><el-button type="primary" @click="handleIOSCreate" :loading="creating">开始</el-button></el-form-item>
        </el-form>
      </div>
    </el-card>

    <!-- 实时数据区域（通用） -->
    <template v-if="currentSession">
      <div class="status-bar">
        <el-tag :type="isIOS ? 'danger' : currentSession.platform === 'android' ? 'warning' : 'success'" size="large">
          {{ isIOS ? '🍎' : currentSession.platform === 'android' ? '📱' : '🖥️' }} 采集中: {{ currentSession.name || currentSession.id }}
        </el-tag>
        <el-tag>{{ isIOS ? currentSession.package : currentSession.platform === 'android' ? currentSession.package : 'PID:' + currentSession.pid }}</el-tag>
        <el-tag>采样 {{ sampleCount }} 条</el-tag>
        <el-button type="danger" size="small" @click="handleStop">⏹ 停止</el-button>
      </div>

      <!-- CPU & Memory -->
      <div class="row2">
        <el-card><template #header><div class="ch"><span>🖥️ CPU</span><span class="lv" :class="cpuClass">{{ latestCPU.toFixed(1) }}%</span></div></template><div ref="cpuChart" class="csm"></div></el-card>
        <el-card><template #header><div class="ch"><span>💾 内存</span><span class="lv">{{ latestMem.toFixed(1) }} MB</span></div></template><div ref="memChart" class="csm"></div></el-card>
      </div>

      <!-- GPU -->
      <div class="row2" v-if="hasGPU">
        <el-card><template #header><div class="ch"><span>🎮 GPU</span><span class="lv">{{ latestGPU.toFixed(1) }}% {{ latestGPUTemp > 0 ? latestGPUTemp.toFixed(0) + '℃' : '' }}</span></div></template><div ref="gpuChart" class="csm"></div></el-card>
        <el-card><template #header><div class="ch"><span>📊 GPU 显存</span><span class="lv">{{ latestGPUMem.toFixed(0) }} MB</span></div></template><div ref="gpuMemChart" class="csm"></div></el-card>
      </div>

      <!-- FPS & Frame Time -->
      <div class="row2" v-if="hasFPS">
        <el-card><template #header><div class="ch"><span>🎞️ FPS</span><span class="lv" :class="fpsClass">{{ latestFPS.toFixed(1) }}</span></div></template><div ref="fpsChart" class="csm"></div></el-card>
        <el-card><template #header><div class="ch"><span>⏱️ 帧时间</span><span class="lv">{{ latestFT.toFixed(2) }} ms</span></div></template><div ref="ftChart" class="csm"></div></el-card>
      </div>

      <!-- Battery & Thermal (Android/iOS) -->
      <div class="row2" v-if="showBattery">
        <el-card><template #header><div class="ch"><span>🔋 电池</span><span class="lv">{{ latestBatteryLevel.toFixed(0) }}% {{ latestBatteryTemp.toFixed(1) }}℃ {{ latestBatteryPower.toFixed(0) }}mW</span></div></template><div ref="batteryChart" class="csm"></div></el-card>
        <el-card><template #header><div class="ch"><span>🌡️ 温度</span><span class="lv">CPU {{ latestCPUTemp.toFixed(1) }}℃ GPU {{ latestGPUTemp.toFixed(1) }}℃</span></div></template><div ref="tempChart" class="csm"></div></el-card>
      </div>

      <!-- Disk & Network -->
      <div class="row2">
        <el-card><template #header><div class="ch"><span>💿 磁盘</span><span class="lv">{{ fmtBPS(latestDiskR) }} / {{ fmtBPS(latestDiskW) }}</span></div></template><div ref="diskChart" class="csm"></div></el-card>
        <el-card><template #header><div class="ch"><span>🌐 网络</span><span class="lv">↑{{ fmtBPS(latestNetS) }} ↓{{ fmtBPS(latestNetR) }}</span></div></template><div ref="netChart" class="csm"></div></el-card>
      </div>
    </template>

    <!-- 历史 -->
    <el-card style="margin-top:20px">
      <template #header><span>📋 最近记录</span></template>
      <el-table :data="recentSessions" stripe max-height="300">
        <el-table-column label="平台" width="60">
          <template #default="{ row }">{{ row.platform === 'android' ? '📱' : row.platform === 'ios' ? '🍎' : '🖥️' }}</template>
        </el-table-column>
        <el-table-column prop="name" label="标签" width="150" />
        <el-table-column label="目标" width="200">
          <template #default="{ row }">{{ row.platform === 'ios' ? row.package : row.platform === 'android' ? row.package : 'PID:' + row.pid }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="80">
          <template #default="{ row }"><el-tag :type="row.status==='running'?'success':'info'" size="small">{{ row.status }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="start_time" label="时间" width="180" />
        <el-table-column label="操作" width="180">
          <template #default="{ row }">
            <el-button size="small" type="primary" @click="$router.push('/compare?highlight='+row.id)">对比</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import { createSession, startCollect, stopCollect, listSessions, getSamples, deleteSession, listAndroidDevices, listAndroidPackages, listIOSDevicesAPI, listIOSAppsAPI, checkIOSPrereqs } from '../api'
import { ElMessage } from 'element-plus'

const platform = ref('windows')
const form = ref({ name: '', pid: null, interval: '1s' })
const androidForm = ref({ name: '', package: '', device_id: '', interval: '2s' })
const iosForm = ref({ name: '', package: '', device_id: '', interval: '2s' })
const creating = ref(false)
const devices = ref([])
const packages = ref([])
const iosDevices = ref([])
const iosApps = ref([])
const iosReady = ref(true)
const currentSession = ref(null)
const recentSessions = ref([])
const sampleCount = ref(0)
const allSamples = ref([])

const cpuChart=ref(null),memChart=ref(null),gpuChart=ref(null),gpuMemChart=ref(null)
const fpsChart=ref(null),ftChart=ref(null),diskChart=ref(null),netChart=ref(null)
const batteryChart=ref(null),tempChart=ref(null)
let charts={},pollTimer=null

const last=()=>allSamples.value.length?allSamples.value[allSamples.value.length-1]:{}
const latestCPU=computed(()=>last().cpu||0)
const latestMem=computed(()=>last().memory||0)
const latestGPU=computed(()=>last().gpu_util||0)
const latestGPUTemp=computed(()=>last().gpu_temp||0)
const latestGPUMem=computed(()=>last().gpu_mem||0)
const latestFPS=computed(()=>last().fps||0)
const latestFT=computed(()=>last().frame_time||0)
const latestDiskR=computed(()=>last().disk_read_bps||0)
const latestDiskW=computed(()=>last().disk_write_bps||0)
const latestNetS=computed(()=>last().net_sent_bps||0)
const latestNetR=computed(()=>last().net_recv_bps||0)
const latestBatteryLevel=computed(()=>last().battery_level||0)
const latestBatteryTemp=computed(()=>last().battery_temp||0)
const latestBatteryPower=computed(()=>last().battery_power||0)
const latestCPUTemp=computed(()=>last().cpu_temp||0)

const hasGPU=computed(()=>allSamples.value.some(s=>s.gpu_util>0))
const hasFPS=computed(()=>allSamples.value.some(s=>s.fps>0))
const isAndroid=computed(()=>currentSession.value?.platform==='android')
const isIOS=computed(()=>currentSession.value?.platform==='ios')
const showBattery=computed(()=>isAndroid.value||isIOS.value)
const cpuClass=computed(()=>{const v=latestCPU.value;return v>80?'danger':v>50?'warning':'good'})
const fpsClass=computed(()=>{const v=latestFPS.value;return v<30?'danger':v<60?'warning':'good'})

function fmtBPS(b){if(!b)return'0 B/s';if(b>1048576)return(b/1048576).toFixed(1)+' MB/s';if(b>1024)return(b/1024).toFixed(1)+' KB/s';return b.toFixed(0)+' B/s'}
function mkOpt(c,u){return{backgroundColor:'transparent',grid:{top:10,right:15,bottom:25,left:55},xAxis:{type:'category',data:[],axisLabel:{color:'#555',fontSize:10}},yAxis:{type:'value',axisLabel:{color:'#555',fontSize:10},splitLine:{lineStyle:{color:'#1a1a3a'}}},series:[{type:'line',smooth:true,showSymbol:false,data:[],itemStyle:{color:c},areaStyle:{color:new echarts.graphic.LinearGradient(0,0,0,1,[{offset:0,color:c+'40'},{offset:1,color:c+'00'}])}}],tooltip:{trigger:'axis'},animation:false}}

function initCharts(){
  const init=(ref,key,c,u)=>{charts[key]=echarts.init(ref.value,'dark');charts[key].setOption(mkOpt(c,u))}
  init(cpuChart,'cpu','#00d4ff','%')
  init(memChart,'mem','#36d399','MB')
  init(gpuChart,'gpu','#a855f7','%')
  init(gpuMemChart,'gpuMem','#f472b6','MB')
  init(fpsChart,'fps','#22d3ee','')
  init(ftChart,'ft','#fb923c','ms')
  init(batteryChart,'bat','#facc15','%')
  init(tempChart,'temp','#ef4444','℃')

  charts.disk=echarts.init(diskChart.value,'dark')
  charts.disk.setOption({backgroundColor:'transparent',grid:{top:30,right:15,bottom:25,left:55},legend:{data:['读取','写入'],textStyle:{color:'#888',fontSize:10},top:0},xAxis:{type:'category',data:[],axisLabel:{color:'#555',fontSize:10}},yAxis:{type:'value',axisLabel:{color:'#555',fontSize:10},splitLine:{lineStyle:{color:'#1a1a3a'}}},series:[{name:'读取',type:'line',smooth:true,showSymbol:false,data:[],itemStyle:{color:'#60a5fa'}},{name:'写入',type:'line',smooth:true,showSymbol:false,data:[],itemStyle:{color:'#f87171'}}],tooltip:{trigger:'axis'},animation:false})
  charts.net=echarts.init(netChart.value,'dark')
  charts.net.setOption({backgroundColor:'transparent',grid:{top:30,right:15,bottom:25,left:55},legend:{data:['上传','下载'],textStyle:{color:'#888',fontSize:10},top:0},xAxis:{type:'category',data:[],axisLabel:{color:'#555',fontSize:10}},yAxis:{type:'value',axisLabel:{color:'#555',fontSize:10},splitLine:{lineStyle:{color:'#1a1a3a'}}},series:[{name:'上传',type:'line',smooth:true,showSymbol:false,data:[],itemStyle:{color:'#34d399'}},{name:'下载',type:'line',smooth:true,showSymbol:false,data:[],itemStyle:{color:'#818cf8'}}],tooltip:{trigger:'axis'},animation:false})
}

function upd(){
  if(!charts.cpu)return
  const s=allSamples.value,t=s.map(x=>`${Math.floor(x.elapsed/60)}:${String(Math.floor(x.elapsed%60)).padStart(2,'0')}`)
  charts.cpu.setOption({xAxis:{data:t},series:[{data:s.map(x=>x.cpu.toFixed(1))}]})
  charts.mem.setOption({xAxis:{data:t},series:[{data:s.map(x=>x.memory.toFixed(1))}]})
  charts.gpu.setOption({xAxis:{data:t},series:[{data:s.map(x=>x.gpu_util.toFixed(1))}]})
  charts.gpuMem.setOption({xAxis:{data:t},series:[{data:s.map(x=>x.gpu_mem.toFixed(0))}]})
  charts.fps.setOption({xAxis:{data:t},series:[{data:s.map(x=>x.fps.toFixed(1))}]})
  charts.ft.setOption({xAxis:{data:t},series:[{data:s.map(x=>x.frame_time.toFixed(2))}]})
  charts.disk.setOption({xAxis:{data:t},series:[{data:s.map(x=>x.disk_read_bps.toFixed(0))},{data:s.map(x=>x.disk_write_bps.toFixed(0))}]})
  charts.net.setOption({xAxis:{data:t},series:[{data:s.map(x=>x.net_sent_bps.toFixed(0))},{data:s.map(x=>x.net_recv_bps.toFixed(0))}]})
  if(charts.bat)charts.bat.setOption({xAxis:{data:t},series:[{data:s.map(x=>x.battery_level.toFixed(0))}]})
  if(charts.temp)charts.temp.setOption({xAxis:{data:t},series:[{data:s.map(x=>x.cpu_temp.toFixed(1))}]})
}

async function poll(){if(!currentSession.value)return;try{const{data}=await getSamples(currentSession.value.id);allSamples.value=data;sampleCount.value=data.length;upd()}catch(e){console.error(e)}}

async function handleCreate(){
  if(!form.value.pid){ElMessage.warning('输入PID');return}
  creating.value=true
  try{
    const{data:s}=await createSession({name:form.value.name,pid:form.value.pid,platform:'windows'})
    await startCollect(s.id,{pid:form.value.pid,interval:form.value.interval})
    currentSession.value={...s,status:'running',platform:'windows'};allSamples.value=[]
    await nextTick();if(!charts.cpu)initCharts();pollTimer=setInterval(poll,2000);ElMessage.success('采集已开始')
  }catch(e){ElMessage.error('失败: '+(e.response?.data?.error||e.message))}
  creating.value=false
}

async function handleAndroidCreate(){
  if(!androidForm.value.package){ElMessage.warning('选择包名');return}
  creating.value=true
  try{
    const{data:s}=await createSession({name:androidForm.value.name,platform:'android',package:androidForm.value.package,device_id:androidForm.value.device_id})
    await startCollect(s.id,{package:androidForm.value.package,device_id:androidForm.value.device_id,interval:androidForm.value.interval})
    currentSession.value={...s,status:'running',platform:'android',package:androidForm.value.package};allSamples.value=[]
    await nextTick();if(!charts.cpu)initCharts();pollTimer=setInterval(poll,2000);ElMessage.success('Android 采集已开始')
  }catch(e){ElMessage.error('失败: '+(e.response?.data?.error||e.message))}
  creating.value=false
}

async function handleIOSCreate(){
  if(!iosForm.value.package){ElMessage.warning('输入 Bundle ID');return}
  creating.value=true
  try{
    const{data:s}=await createSession({name:iosForm.value.name,platform:'ios',package:iosForm.value.package,device_id:iosForm.value.device_id})
    await startCollect(s.id,{package:iosForm.value.package,device_id:iosForm.value.device_id,interval:iosForm.value.interval})
    currentSession.value={...s,status:'running',platform:'ios',package:iosForm.value.package};allSamples.value=[]
    await nextTick();if(!charts.cpu)initCharts();pollTimer=setInterval(poll,2000);ElMessage.success('iOS 采集已开始')
  }catch(e){ElMessage.error('失败: '+(e.response?.data?.error||e.message))}
  creating.value=false
}

async function handleStop(){
  if(!currentSession.value)return
  await stopCollect(currentSession.value.id);clearInterval(pollTimer);currentSession.value=null;ElMessage.success('已停止');loadSessions()
}
async function handleDelete(id){await deleteSession(id);ElMessage.success('已删除');loadSessions()}
async function loadSessions(){try{const{data}=await listSessions();recentSessions.value=data.slice(0,20)}catch{}}
async function loadDevices(){try{const{data}=await listAndroidDevices();devices.value=data.devices||[]}catch{ElMessage.warning('ADB 不可用')}}
async function loadPackages(){if(!androidForm.value.device_id&&devices.value.length===0)await loadDevices();try{const{data}=await listAndroidPackages(androidForm.value.device_id||'');packages.value=data.packages||[]}catch{}}
async function loadIOSDevices(){try{const{data}=await checkIOSPrereqs();iosReady.value=data.ready;if(!data.ready){ElMessage.warning(data.error);return}const{data:d}=await listIOSDevicesAPI();iosDevices.value=(d.devices||[]).map(dev=>({udid:dev.Identifier||dev.DeviceIdentifier||'',name:dev.DeviceName||'Unknown',version:dev.ProductVersion||'',product:dev.ProductType||''}))}catch(e){ElMessage.warning('pymobiledevice3 不可用: '+e.message)}}
async function loadIOSApps(){if(!iosForm.value.device_id&&iosDevices.value.length===0)await loadIOSDevices();try{const{data}=await listIOSAppsAPI(iosForm.value.device_id||'');iosApps.value=data.apps||[]}catch(e){ElMessage.warning('获取应用列表失败')}}

onMounted(loadSessions)
onUnmounted(()=>{clearInterval(pollTimer);Object.values(charts).forEach(c=>c?.dispose())})
</script>

<style scoped>
.status-bar{display:flex;align-items:center;gap:10px;margin:12px 0;flex-wrap:wrap}
.row2{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:12px}
.ch{display:flex;justify-content:space-between;align-items:center}
.lv{font-size:18px;font-weight:700;color:#00d4ff}
.lv.warning{color:#f59e0b}.lv.danger{color:#ef4444}.lv.good{color:#36d399}
.csm{height:200px}
</style>
