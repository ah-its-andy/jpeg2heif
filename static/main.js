async function fetchJSON(url, opts={}){ const r = await fetch(url, opts); if(!r.ok) throw new Error(await r.text()); return await r.json(); }

async function loadStats(){
  try{
    const s = await fetchJSON('/api/stats');
    document.getElementById('stats').innerHTML = `
      <b>Watcher:</b> ${s.watcher_state} | <b>Queue:</b> ${s.queue_len}
      | <b>Total:</b> ${s.total} | <b>Success:</b> ${s.success} | <b>Failed:</b> ${s.failed}
      | <b>Meta preserved rate:</b> ${(s.metadata_preserve_rate*100).toFixed(1)}%
    `;
  }catch(e){ console.error(e); }
}

async function loadFiles(){
  const st = document.getElementById('statusFilter').value;
  const url = '/api/files?limit=100' + (st?`&status=${encodeURIComponent(st)}`:'');
  const d = await fetchJSON(url);
  const tbody = document.querySelector('#filesTbl tbody');
  tbody.innerHTML='';
  d.data.forEach(row=>{
    const tr = document.createElement('tr');
    tr.innerHTML = `
      <td>${row.id}</td>
      <td title="${row.file_path}">${row.file_path}</td>
      <td><span class="badge ${row.status}">${row.status}</span></td>
      <td>${row.file_md5}</td>
      <td>${row.metadata_summary||''}</td>
      <td>${row.updated_at}</td>
    `;
    tbody.appendChild(tr);
  });
}

let currJobId = null;
async function startRebuild(){
  try{
    const r = await fetchJSON('/api/rebuild-index', {method:'POST'});
    currJobId = r.job_id; document.getElementById('jobStatus').textContent = 'Rebuild started: '+currJobId;
  }catch(e){ alert('start rebuild failed: '+e);}
}
async function pollJob(){
  if(!currJobId) return;
  try{
    const j = await fetchJSON('/api/rebuild-status/'+currJobId);
    document.getElementById('jobStatus').textContent = `Rebuild ${j.status}: ${j.indexed}/${j.total}`;
    if(j.status==='done') currJobId=null;
  }catch(e){ console.warn(e); currJobId=null; }
}

async function scanNow(){
  try{ await fetchJSON('/api/scan-now', {method:'POST'}); }catch(e){ alert('scan failed: '+e); }
}

document.getElementById('rebuildBtn').onclick = startRebuild;

document.getElementById('refreshBtn').onclick = ()=>{loadStats(); loadFiles();};

document.getElementById('scanBtn').onclick = scanNow;

document.getElementById('statusFilter').onchange = loadFiles;

setInterval(()=>{loadStats(); pollJob();}, 2000);

loadStats(); loadFiles();
