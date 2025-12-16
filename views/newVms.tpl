<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Create New VMs</title>
    <link rel="stylesheet" href="/static/css/style.css">
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css" rel="stylesheet">
    <style>
        body {
            font-family:'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            margin:0;
            padding:0;
            background:#0d1117;
            color:#c9d1d9;
            display:flex;
            flex-direction:column;
            min-height:100vh;
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
            transition: color 0.2s ease;
        }
        nav a:hover { color:#1f6feb; }
        nav a.active { color:#ff7b72; }
        .main-content {
            flex:1;
            padding:40px 20px;
            max-width:1000px;
            margin:0 auto;
            text-align:center;
        }
        .dashboard { 
            display:flex; 
            flex-wrap:wrap; 
            justify-content:center; 
            gap:20px; 
            margin-top:20px; 
        }
        .card {
            background:#161b22;
            border-radius:12px;
            padding:20px;
            width:280px;
            text-align:left;
            box-shadow:0 4px 12px rgba(0,0,0,0.5);
            transition: transform 0.2s ease, box-shadow 0.2s ease;
            border:1px solid #30363d;
        }
        .card:hover { transform: translateY(-5px); box-shadow:0 8px 20px rgba(0,0,0,0.6); }
        .card h3 { 
            margin-bottom:10px; 
            color:#58a6ff; 
            text-align:center;
            display:flex;
            align-items:center;
            justify-content:center;
            gap:5px;
        }
        .card input, .card select {
            width:100%;
            padding:8px;
            margin:5px 0;
            border-radius:6px;
            border:1px solid #30363d;
            background:#0d1117;
            color:#c9d1d9;
            box-sizing:border-box;
        }
        .card input:focus, .card select:focus {
            border-color:#58a6ff;
            outline:none;
        }
        .card button.remove-vm {
            width:100%;
            margin-top:10px;
            padding:6px 12px;
            cursor:pointer;
            border:none;
            border-radius:8px;
            background:#da3633;
            color:#fff;
            font-weight:bold;
        }
        .card button.remove-vm:hover { background:#f85149; }
        .controls { margin:20px 0; }
        #btnAddVm {
            background:#58a6ff;
            color:#fff;
            border:none;
            border-radius:8px;
            padding:10px 20px;
            font-weight:bold;
            cursor:pointer;
            transition: background 0.2s;
        }
        #btnAddVm:hover { background:#1f6feb; }
        #vmsInfoSubmit {
            background:#238636;
            color:#fff;
            border:none;
            border-radius:8px;
            padding:12px 24px;
            font-weight:bold;
            cursor:pointer;
            margin-top:20px;
            font-size:1.1rem;
        }
        #vmsInfoSubmit:hover { background:#2ea043; }
        #vmsInfoSubmit:disabled { background:#8b949e; cursor:not-allowed; }
        .alert-warning {
            background:#f0c14b;
            color:#000;
            padding:10px;
            border-radius:6px;
            margin:10px 0;
            border:1px solid #f0c14b;
        }
        @media(max-width:600px){
            .card { width:90%; padding:15px; }
            nav a { margin:0 10px; font-size:14px; }
            .dashboard { flex-direction:column; align-items:center; }
        }
    </style>
</head>
<body>
    <header>
        <h1><i class="fas fa-server"></i> EM Controller</h1>
        <p>Create New Virtual Machines</p>
    </header>
    <nav>
        <a href="/">Home</a>
        <a href="/application">Application</a>
        <a href="/cloud">Cloud</a>
        <a href="/vm" class="active">VM</a>
        <a href="/k8sNode">K8s Node</a>
        <a href="/container">Container</a>
        <a href="/image">Image</a>
        <a href="/network">Network</a>
        <a href="/state">State</a>
    </nav>

    <div class="main-content">
        <div class="alert-warning">
            <i class="fas fa-exclamation-triangle"></i> 
            Note: All names should follow the DNS label standard (RFC 1123): lowercase alphanumeric or "-", start/end with alphanumeric, ≤ 63 chars.
        </div>

        <div class="controls">
            <button type="button" id="btnAddVm">
                <i class="fas fa-plus"></i> Add VM
            </button>
        </div>

        <!-- ĐẶT TẤT CẢ INPUT TRONG FORM -->
        <form id="vmsInfo" action="/vm/doNew" method="post">
            <input type="hidden" id="newVmNum" name="newVmNumber" value="0">
            <div class="dashboard" id="vmsStart"></div>
            <input type="submit" id="vmsInfoSubmit" value="Create VMs" disabled>
        </form>
    </div>

    <script>
    //<![CDATA[
        var vmCount = 0;

        function getVmCard(index) {
            // Tránh dùng template literal/backtick để không vào state JSTmplLit
            var html = '';
            html += '<div class="card" data-vm-index="' + index + '">';
            html +=   '<h3><i class="fas fa-laptop"></i> VM ' + (index + 1) + '</h3>';

            html +=   '<input type="text" name="vm' + index + 'Name" placeholder="VM Name (e.g., my-vm-01)" required maxlength="63">';

            html +=   '<select name="vm' + index + 'CloudName" required>';
            html +=     '<option value="">Select Cloud</option>';
            html +=     '<option value="myvm">Local (myvm)</option>';
            html +=     '<option value="aws">AWS</option>';
            html +=     '<option value="gcp">GCP</option>';
            html +=     '<option value="azure">Azure</option>';
            html +=   '</select>';

            html +=   '<input type="number" name="vm' + index + 'VCpu" placeholder="CPU Cores (e.g., 2)" min="1" step="1" required>';
            html +=   '<input type="number" name="vm' + index + 'Ram" placeholder="Memory (MB, e.g., 2048)" min="256" step="1" required>';
            html +=   '<input type="number" name="vm' + index + 'Storage" placeholder="Storage (GB, e.g., 50)" min="1" step="1" required>';

            html +=   '<input type="text" placeholder="IP Address (optional)" disabled>';

            html +=   '<button type="button" class="remove-vm" onclick="removeVm(' + index + ')">';
            html +=     '<i class="fas fa-trash"></i> Remove VM';
            html +=   '</button>';

            html += '</div>';
            return html;
        }

        document.getElementById('btnAddVm').addEventListener('click', function () {
            var dashboard = document.getElementById('vmsStart');
            dashboard.insertAdjacentHTML('beforeend', getVmCard(vmCount));
            vmCount++;
            syncCountAndButtons();
            reindexVmCards(); // đảm bảo nhất quán
        });

        function removeVm(index) {
            var card = document.querySelector('.card[data-vm-index="' + index + '"]');
            if (card) {
                card.parentNode.removeChild(card);
                vmCount = document.querySelectorAll('.card').length;
                syncCountAndButtons();
                reindexVmCards();
            }
        }

        function reindexVmCards() {
            var cards = document.querySelectorAll('.card');
            cards.forEach(function(card, i) {
                card.setAttribute('data-vm-index', i);
                var title = card.querySelector('h3');
                if (title) title.innerHTML = '<i class="fas fa-laptop"></i> VM ' + (i + 1);

                var inputs = card.querySelectorAll('input, select');
                inputs.forEach(function(el) {
                    if (!el.name) return;
                    el.name = el.name
                        .replace(/vm\d+Name/, 'vm' + i + 'Name')
                        .replace(/vm\d+CloudName/, 'vm' + i + 'CloudName')
                        .replace(/vm\d+VCpu/, 'vm' + i + 'VCpu')
                        .replace(/vm\d+Ram/, 'vm' + i + 'Ram')
                        .replace(/vm\d+Storage/, 'vm' + i + 'Storage');
                });

                var rm = card.querySelector('.remove-vm');
                if (rm) rm.setAttribute('onclick', 'removeVm(' + i + ')');
            });
        }

        function syncCountAndButtons() {
            document.getElementById('newVmNum').value = vmCount;
            document.getElementById('vmsInfoSubmit').disabled = (vmCount === 0);

            var cards = document.querySelectorAll('.card');
            cards.forEach(function(card) {
                var btn = card.querySelector('.remove-vm');
                if (btn) btn.style.display = (cards.length > 1) ? 'block' : 'none';
            });
        }

        document.getElementById('vmsInfo').addEventListener('submit', function (e) {
            var dnsRegex = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

            var nameInputs = document.querySelectorAll('input[name^="vm"][name$="Name"]');
            for (var i = 0; i < nameInputs.length; i++) {
                var v = nameInputs[i].value.trim();
                if (!v) {
                    alert('Name is required.');
                    nameInputs[i].focus();
                    e.preventDefault();
                    return;
                }
                if (!dnsRegex.test(v) || v.length > 63) {
                    alert('Invalid name "' + v + '": Must be lowercase alphanumeric + \'-\', start/end with alphanumeric, max 63 chars.');
                    nameInputs[i].focus();
                    e.preventDefault();
                    return;
                }
            }

            var requiredNumbers = document.querySelectorAll(
                'input[name^="vm"][name$="VCpu"], input[name^="vm"][name$="Ram"], input[name^="vm"][name$="Storage"]'
            );
            for (var j = 0; j < requiredNumbers.length; j++) {
                var num = requiredNumbers[j];
                if (num.value === '' || isNaN(Number(num.value))) {
                    alert('CPU/RAM/Storage must be valid numbers.');
                    num.focus();
                    e.preventDefault();
                    return;
                }
            }
        });
    //]]>
    </script>

    {{template "footer" .}}
</body>
</html>
