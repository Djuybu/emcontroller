<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Cloud - {{.Website}}</title>
    <link rel="stylesheet" href="/static/css/style.css">
    <!-- Thêm Font Awesome cho icons -->
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css" rel="stylesheet">
    <style>
        body {
            font-family:'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            margin:0;
            padding:0;
            display:flex;
            flex-direction:column;
            min-height:100vh;
            background:#0d1117;
            color:#c9d1d9;
        }
        header {
            background: linear-gradient(90deg,#0d1117,#1f1f1f);
            padding:20px 0;
            text-align:center;
        }
        header h1 { margin:0; font-size:2rem; color:#58a6ff; }
        header p { margin:5px 0 0 0; font-size:1rem; color:#8b949e; }
        nav {
            background:#161b22;
            padding:10px 0;
            text-align:center;
        }
        nav a {
            color:#58a6ff;
            margin:0 15px;
            text-decoration:none;
            font-weight:bold;
            transition: color 0.2s;
        }
        nav a:hover { color:#1f6feb; }
        nav a.active { color:#ff7b72; }
        .main-content {
            flex:1;
            padding:40px 20px;
            max-width:1000px;
            margin:0 auto;
        }
        h2, h3 {
            color:#58a6ff;
            text-align:center;
            margin-bottom:20px;
        }
        /* Stats cards */
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .stat-card {
            background: #161b22;
            border-radius: 12px;
            padding: 20px;
            text-align: center;
            box-shadow: 0 4px 12px rgba(0,0,0,0.5);
            transition: transform 0.2s ease;
            border: 1px solid #30363d;
        }
        .stat-card:hover { transform: translateY(-5px); }
        .stat-card i { font-size: 2rem; color: #58a6ff; margin-bottom: 10px; }
        .stat-card h4 { margin: 0 0 10px 0; color: #c9d1d9; }
        .stat-card .value { font-size: 2rem; color: #58a6ff; font-weight: bold; }
        /* Table */
        .table-container {
            overflow-x:auto;
            margin-top:20px;
            background: #161b22;
            border-radius: 12px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.5);
        }
        table {
            width:100%;
            border-collapse:collapse;
            background:#161b22;
        }
        table th, table td {
            padding:12px 15px;
            text-align:left;
            border-bottom:1px solid #272b33;
        }
        table th {
            background:#1f1f1f;
            color:#58a6ff;
            position: sticky;
            top: 0;
        }
        table tr:hover { background:#272b33; }
        table tr:nth-child(even) { background: #0d1117; }
        /* Progress bars cho resources */
        .progress {
            height: 8px;
            background: #30363d;
            border-radius: 4px;
            overflow: hidden;
            margin: 5px 0;
        }
        .progress-bar {
            height: 100%;
            background: linear-gradient(90deg, #238636, #58a6ff);
            transition: width 0.3s ease;
        }
        .usage-badge {
            background: #238636;
            color: #fff;
            padding: 2px 6px;
            border-radius: 4px;
            font-size: 0.8rem;
            margin-left: 5px;
        }
        .badge {
            background: #58a6ff;
            color: #fff;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 0.8rem;
            display: inline-block;
        }
        .bg-primary { background: #58a6ff !important; }
        .empty { color: #8b949e; font-style: italic; }
        /* Buttons */
        .btn {
            background: #58a6ff;
            color: #fff;
            border: none;
            padding: 8px 16px;
            border-radius: 6px;
            cursor: pointer;
            text-decoration: none;
            font-weight: bold;
            transition: background 0.2s;
            display: inline-block;
            margin: 10px 5px;
        }
        .btn:hover { background: #1f6feb; }
        .btn-success { background: #238636; }
        .btn-success:hover { background: #2ea043; }
        .btn-add { margin-bottom: 20px; text-align: center; }
        /* Modal cho Add Cloud */
        .modal { display: none; position: fixed; z-index: 1000; left: 0; top: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.5); }
        .modal-content { background: #161b22; margin: 15% auto; padding: 20px; border-radius: 12px; width: 80%; max-width: 500px; }
        .close { color: #aaa; float: right; font-size: 28px; font-weight: bold; cursor: pointer; }
        .close:hover { color: #fff; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; color: #58a6ff; margin-bottom: 5px; }
        .form-group input, .form-group select { width: 100%; padding: 8px; background: #0d1117; color: #c9d1d9; border: 1px solid #30363d; border-radius: 6px; box-sizing: border-box; }
        @media(max-width:600px){
            table th, table td { padding:8px; }
            nav a { margin:0 10px; }
            .stats-grid { grid-template-columns: 1fr; }
            .modal-content { width: 95%; margin: 10% auto; }
        }
    </style>
</head>
<body>
    <header>
        <h1><i class="fas fa-cloud"></i> {{.Website}}</h1>
        <p>Version: {{.VersionInfo}}</p>
    </header>
    <nav>
        <a href="/">Home</a>
        <a href="/application">Application</a>
        <a href="/cloud" class="active">Cloud</a>
        <a href="/vm">VM</a>
        <a href="/k8sNode">K8s Node</a>
        <a href="/container">Container</a>
        <a href="/image">Image</a>
        <a href="/network">Network</a>
        <a href="/state">State</a>
    </nav>
    <div class="main-content">
        <h2>All Clouds <i class="fas fa-list"></i></h2>
       
        <!-- Stats cards tổng quan (sử dụng data từ controller) -->
        <div class="stats-grid">
            <div class="stat-card">
                <i class="fas fa-server"></i>
                <h4>Total Clouds</h4>
                <div class="value">{{.TotalClouds}}</div>
            </div>
            <div class="stat-card">
                <i class="fas fa-tachometer-alt"></i>
                <h4>Avg CPU Usage</h4>
                <div class="value">{{.AvgCpuUsage}}</div>
            </div>
            <div class="stat-card">
                <i class="fas fa-memory"></i>
                <h4>Total Memory</h4>
                <div class="value">{{.TotalMemory}} MB</div>
            </div>
            <div class="stat-card">
                <i class="fas fa-hdd"></i>
                <h4>Total Storage</h4>
                <div class="value">{{.TotalStorage}} GB</div>
            </div>
        </div>
       
        <!-- Button Add Cloud -->
        <div class="btn-add">
            <button class="btn btn-success" onclick="openModal()">
                <i class="fas fa-plus"></i> Add Cloud
            </button>
        </div>
       
        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th rowspan="2">Name</th>
                        <th rowspan="2">Type</th>
                        <th rowspan="2">Web URL</th>
                        <!-- NEW: Nhiệt độ -->
                        <th rowspan="2">Temperature (°C)</th>
                        <th colspan="6">Resources (used/total)</th>
                    </tr>
                    <tr>
                        <th>CPU</th>
                        <th>Memory (MB)</th>
                        <th>Storage (GB)</th>
                        <th>VM</th>
                        <th>Volume</th>
                        <th>Network Port</th>
                    </tr>
                </thead>
                <tbody>
                    {{range $cloudIdx, $cloud := .cloudList}}
                    <tr>
                        <td><a href="/cloud/{{$cloud.Name}}"><i class="fas fa-tag"></i> {{$cloud.Name}}</a></td>
                        <td><span class="badge bg-primary">{{$cloud.Type}}</span></td>
                        <td><a href="{{$cloud.WebUrl}}" target="_blank" rel="noopener noreferrer"><i class="fas fa-external-link-alt"></i> {{$cloud.WebUrl}}</a></td>

                        <!-- NEW: Cột nhiệt độ -->
                        <td>
                            {{if $cloud.HasTemperature}}
                                <i class="fas fa-temperature-half"></i>
                                {{printf "%.1f" $cloud.TemperatureC}}&deg;C
                            {{else}}
                                <span class="empty">N/A</span>
                            {{end}}
                        </td>

                        {{if lt $cloud.Resources.Limit.VCpu 0.0}}
                            <td class="empty">N/A</td>
                        {{else}}
                            <td>
                                <span class="usage-badge">{{$cloud.Resources.InUse.VCpu}}/{{$cloud.Resources.Limit.VCpu}}</span>
                                <div class="progress">
                                    <div class="progress-bar" style="width: {{$cloud.CpuPercent}}%;"></div>
                                </div>
                            </td>
                        {{end}}
                        {{if lt $cloud.Resources.Limit.Ram 0.0}}
                            <td class="empty">N/A</td>
                        {{else}}
                            <td>
                                <span class="usage-badge">{{$cloud.Resources.InUse.Ram}}/{{$cloud.Resources.Limit.Ram}}</span>
                                <div class="progress">
                                    <div class="progress-bar" style="width: {{$cloud.RamPercent}}%;"></div>
                                </div>
                            </td>
                        {{end}}
                        {{if lt $cloud.Resources.Limit.Storage 0.0}}
                            <td class="empty">N/A</td>
                        {{else}}
                            <td>
                                <span class="usage-badge">{{$cloud.Resources.InUse.Storage}}/{{$cloud.Resources.Limit.Storage}}</span>
                                <div class="progress">
                                    <div class="progress-bar" style="width: {{$cloud.StoragePercent}}%;"></div>
                                </div>
                            </td>
                        {{end}}
                        {{if lt $cloud.Resources.Limit.Vm 0.0}}
                            <td class="empty">N/A</td>
                        {{else}}
                            <td>
                                <span class="usage-badge">{{$cloud.Resources.InUse.Vm}}/{{$cloud.Resources.Limit.Vm}}</span>
                                <div class="progress">
                                    <div class="progress-bar" style="width: {{$cloud.VmPercent}}%;"></div>
                                </div>
                            </td>
                        {{end}}
                        {{if lt $cloud.Resources.Limit.Volume 0.0}}
                            <td class="empty">N/A</td>
                        {{else}}
                            <td>
                                <span class="usage-badge">{{$cloud.Resources.InUse.Volume}}/{{$cloud.Resources.Limit.Volume}}</span>
                                <div class="progress">
                                    <div class="progress-bar" style="width: {{$cloud.VolumePercent}}%;"></div>
                                </div>
                            </td>
                        {{end}}
                        {{if lt $cloud.Resources.Limit.Port 0.0}}
                            <td class="empty">N/A</td>
                        {{else}}
                            <td>
                                <span class="usage-badge">{{$cloud.Resources.InUse.Port}}/{{$cloud.Resources.Limit.Port}}</span>
                                <div class="progress">
                                    <div class="progress-bar" style="width: {{$cloud.PortPercent}}%;"></div>
                                </div>
                            </td>
                        {{end}}
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
   
    <!-- Modal Add Cloud -->
    <div id="addCloudModal" class="modal">
        <div class="modal-content">
            <span class="close" onclick="closeModal()">&times;</span>
            <h3><i class="fas fa-plus"></i> Add New Cloud</h3>
            <form id="addCloudForm">
                <div class="form-group">
                    <label for="cloudName">Name</label>
                    <input type="text" id="cloudName" name="name" placeholder="e.g., My-AWS-Cloud" required>
                </div>
                <div class="form-group">
                    <label for="cloudType">Type</label>
                    <select id="cloudType" name="type" required>
                        <option value="">Select Type</option>
                        <option value="AWS">AWS</option>
                        <option value="GCP">GCP</option>
                        <option value="Azure">Azure</option>
                    </select>
                </div>
                <div class="form-group">
                    <label for="webUrl">Web URL</label>
                    <input type="url" id="webUrl" name="webUrl" placeholder="https://console.aws.amazon.com" required>
                </div>
                <button type="submit" class="btn btn-success">Add Cloud</button>
            </form>
        </div>
    </div>
   
    <!-- JS cho Modal & Form (AJAX POST đến /cloud/add) -->
    <script>
        function openModal() {
            document.getElementById('addCloudModal').style.display = 'block';
        }
        function closeModal() {
            document.getElementById('addCloudModal').style.display = 'none';
        }
        window.onclick = function(event) {
            const modal = document.getElementById('addCloudModal');
            if (event.target == modal) { closeModal(); }
        }
        // Submit form với AJAX
        document.getElementById('addCloudForm').addEventListener('submit', function(e) {
            e.preventDefault();
            const formData = new FormData(this);
            fetch('/cloud/add', {
                method: 'POST',
                body: formData
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    alert(data.message);
                    closeModal();
                    location.reload(); // Reload để update table
                } else {
                    alert('Error: ' + data.error);
                }
            })
            .catch(error => {
                alert('Error adding cloud: ' + error);
            });
        });
    </script>
   
    {{template "footer" .}}
</body>
</html>
