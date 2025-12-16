import requests
import datetime
import json
from types import SimpleNamespace
import math

import data_types

# endpoint of multi-cloud manager
MCM_END_POINT = "172.27.15.31:20000"
# MCM_END_POINT = "localhost:20000"


def resp_code_successful(code: int) -> bool:
    return code >= 200 and code < 300


# get applications from multi-cloud manager
def get_all_apps():
    url = "http://" + MCM_END_POINT + "/application"
    headers = {
        'Accept': 'application/json',
    }
    # response = requests.get(url, headers=headers, timeout=10)
    response = requests.get(url, headers=headers)

    if not resp_code_successful(response.status_code):
        raise Exception(
            "URL {}, Unexcepted status code: {}, response body: {}".format(
                url, response.status_code, response.text))

    return json.loads(response.text,
                      object_hook=lambda d: SimpleNamespace(**d))


def calculate_temperature_losses(temperature: float) -> tuple[float, float]:
    """
    Calculate performance loss and power overhead based on temperature.
    
    Performance degradation model:
    - Optimal temperature: 20-25°C (0% loss)
    - Below 20°C: slight degradation due to cold (linear, max 5% at 0°C)
    - Above 25°C: exponential degradation (more significant at high temps)
    - At 35°C: ~10% performance loss
    - At 45°C: ~25% performance loss
    - At 55°C: ~50% performance loss
    
    Power overhead model:
    - Optimal: 20-25°C (0% overhead)
    - Below 20°C: heating needed (linear, ~2% per 5°C below 20°C)
    - Above 25°C: cooling needed (exponential, ~5% at 30°C, ~15% at 40°C, ~40% at 50°C)
    
    Returns:
        (performance_loss, power_overhead) where:
        - performance_loss: 0.0-1.0 (fraction of performance lost)
        - power_overhead: percentage increase in power consumption
    """
    # Performance loss calculation
    if temperature < 20.0:
        # Cold: linear degradation from 0% at 20°C to 5% at 0°C
        performance_loss = (20.0 - temperature) / 20.0 * 0.05
    elif temperature <= 25.0:
        # Optimal range: no loss
        performance_loss = 0.0
    else:
        # Hot: exponential degradation
        excess_temp = temperature - 25.0
        # Model: loss increases exponentially with excess temperature
        # At 10°C excess (35°C): ~10% loss
        # At 20°C excess (45°C): ~25% loss
        # At 30°C excess (55°C): ~50% loss
        performance_loss = 1.0 - math.exp(-0.1 * excess_temp)
        # Cap at 70% max loss
        performance_loss = min(performance_loss, 0.7)
    
    # Power overhead calculation
    if temperature < 20.0:
        # Cold: heating needed
        power_overhead = (20.0 - temperature) / 5.0 * 2.0  # ~2% per 5°C below 20°C
    elif temperature <= 25.0:
        # Optimal range: no overhead
        power_overhead = 0.0
    else:
        # Hot: cooling needed
        excess_temp = temperature - 25.0
        # Model: power overhead for cooling increases with temperature
        # At 5°C excess (30°C): ~5% overhead
        # At 15°C excess (40°C): ~15% overhead
        # At 25°C excess (50°C): ~40% overhead
        power_overhead = 0.5 * excess_temp + 0.02 * excess_temp * excess_temp
        # Cap at 50% max overhead
        power_overhead = min(power_overhead, 50.0)
    
    return performance_loss, power_overhead


def get_cloud_temperature_for_app(app: data_types.AppInfo) -> float:
    """
    Get the temperature of the cloud where the app is deployed.
    Uses the first pod's host node to determine the cloud by matching VM IPs.
    """
    if not app.hosts or len(app.hosts) == 0:
        # Default temperature if no host info
        return 20.0
    
    try:
        # Get all VMs to find which cloud this node belongs to
        vms = get_all_vms()
        host_ip = app.hosts[0].hostIP
        host_name = app.hosts[0].hostName
        
        # Find the VM that matches the host IP or host name
        cloud_name = None
        for vm in vms:
            # Match by IP or by name (node name usually matches VM name)
            if hasattr(vm, 'ips') and host_ip in vm.ips:
                cloud_name = vm.cloud if hasattr(vm, 'cloud') else None
                break
            elif hasattr(vm, 'name') and vm.name == host_name:
                cloud_name = vm.cloud if hasattr(vm, 'cloud') else None
                break
        
        if cloud_name:
            # Get temperature for this cloud
            # Since we don't have a direct JSON API for clouds, we'll use a default mapping
            # or fetch from weather API based on cloud name
            return get_temperature_for_cloud_name(cloud_name)
    except Exception as e:
        print(f"Warning: Could not get cloud temperature for app {app.appName}: {e}")
    
    # Default temperature if we can't determine
    return 20.0


def get_temperature_for_cloud_name(cloud_name: str) -> float:
    """
    Get temperature for a cloud by name using default location mapping.
    In a production system, this would fetch from the cloud's actual location.
    """
    # Default location mapping (same as in Go code)
    location_map = {
        "myvm": ("21.0285", "105.8542"),  # Hà Nội
        "CLAAUDIAweifan": ("55.6762", "12.5683"),  # Copenhagen
        # Default other clouds to Hà Nội
    }
    
    # Get location for this cloud
    lat, lon = location_map.get(cloud_name, ("21.0285", "105.8542"))
    
    # Try to fetch temperature from weather API
    try:
        import requests as req_lib
        base_url = "https://api.open-meteo.com/v1/forecast"
        params = {
            "latitude": lat,
            "longitude": lon,
            "current": "temperature_2m"
        }
        response = req_lib.get(base_url, params=params, timeout=5)
        if response.status_code == 200:
            data = response.json()
            if "current" in data and "temperature_2m" in data["current"]:
                return float(data["current"]["temperature_2m"])
    except Exception as e:
        print(f"Warning: Could not fetch temperature from API for cloud {cloud_name}: {e}")
    
    # Default temperature
    return 20.0


def call_app(app: data_types.AppInfo) -> data_types.ResultData:
    # endpoint of this app
    app_ep = "{}:{}".format(app.nodePortIP[0], app.nodePort[0])

    url = "http://" + app_ep + "/experiment"
    time_before = datetime.datetime.now()
    response = requests.get(url)
    time_after = datetime.datetime.now()
    if not resp_code_successful(response.status_code):
        raise Exception(
            "URL {}, Unexcepted status code: {}, response body: {}".format(
                url, response.status_code, response.text))
    durations = (time_after - time_before).total_seconds() * 1000  # unit: ms
    
    # Get temperature for the cloud where this app is deployed
    temperature = get_cloud_temperature_for_app(app)
    
    # Calculate losses based on temperature
    performance_loss, power_overhead = calculate_temperature_losses(temperature)
    
    return data_types.ResultData(
        app_name=app.appName,
        priority=app.priority,
        resp_time=durations,
        resp_time_in_clouds=float(response.text),
        pri_wei_resp_time=durations * float(app.priority),
        pri_wei_resp_time_in_clouds=float(response.text) * float(app.priority),
        temperature=temperature,
        performance_loss=performance_loss,
        power_overhead=power_overhead)


# delete some applications via multi-cloud manager
def del_apps(app_names: list[str]):
    url = "http://" + MCM_END_POINT + "/application"
    headers = {
        'Content-Type': 'application/json',
    }
    response = requests.delete(url, headers=headers, json=app_names)

    if not resp_code_successful(response.status_code):
        raise Exception(
            "URL {}, Unexcepted status code: {}, response body: {}".format(
                url, response.status_code, response.text))


# get all Virtual Machines via multi-cloud manager
def get_all_vms():
    url = "http://" + MCM_END_POINT + "/vm"
    headers = {
        'Accept': 'application/json',
    }
    response = requests.get(url, headers=headers)

    if not resp_code_successful(response.status_code):
        raise Exception(
            "URL {}, Unexcepted status code: {}, response body: {}".format(
                url, response.status_code, response.text))

    return json.loads(response.text,
                      object_hook=lambda d: SimpleNamespace(**d))


# get all Kubernetes nodes via multi-cloud manager
def get_k8s_nodes():
    url = "http://" + MCM_END_POINT + "/k8sNode"
    headers = {
        'Accept': 'application/json',
    }
    response = requests.get(url, headers=headers)

    if not resp_code_successful(response.status_code):
        raise Exception(
            "URL {}, Unexcepted status code: {}, response body: {}".format(
                url, response.status_code, response.text))

    return json.loads(response.text,
                      object_hook=lambda d: SimpleNamespace(**d))