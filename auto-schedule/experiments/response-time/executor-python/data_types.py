from dataclasses import dataclass


# with @dataclass, we do not need to implement the init function of a Class
@dataclass
class ResultData:
    app_name: str
    priority: int
    resp_time: float
    resp_time_in_clouds: float
    pri_wei_resp_time: float  # priority weighted response time
    pri_wei_resp_time_in_clouds: float  # priority weighted response time consumed in clouds
    temperature: float  # temperature in Celsius at the cloud where app is deployed
    performance_loss: float  # performance degradation factor (0.0-1.0) due to temperature
    power_overhead: float  # power consumption overhead percentage due to temperature


# calculate the average value of a given list of ResultData
def calc_rd_avg(data_list: list[ResultData]) -> ResultData:
    app_name = data_list[0].app_name
    priority = data_list[0].priority

    # calculate the average values
    resp_time = 0.0
    resp_time_in_clouds = 0.0
    pri_wei_resp_time = 0.0
    pri_wei_resp_time_in_clouds = 0.0
    temperature = 0.0
    performance_loss = 0.0
    power_overhead = 0.0
    for _, one_data in enumerate(data_list):
        resp_time += one_data.resp_time
        resp_time_in_clouds += one_data.resp_time_in_clouds
        pri_wei_resp_time += one_data.pri_wei_resp_time
        pri_wei_resp_time_in_clouds += one_data.pri_wei_resp_time_in_clouds
        temperature += one_data.temperature
        performance_loss += one_data.performance_loss
        power_overhead += one_data.power_overhead
    resp_time /= len(data_list)
    resp_time_in_clouds /= len(data_list)
    pri_wei_resp_time /= len(data_list)
    pri_wei_resp_time_in_clouds /= len(data_list)
    temperature /= len(data_list)
    performance_loss /= len(data_list)
    power_overhead /= len(data_list)

    return ResultData(app_name=app_name,
                      priority=priority,
                      resp_time=resp_time,
                      resp_time_in_clouds=resp_time_in_clouds,
                      pri_wei_resp_time=pri_wei_resp_time,
                      pri_wei_resp_time_in_clouds=pri_wei_resp_time_in_clouds,
                      temperature=temperature,
                      performance_loss=performance_loss,
                      power_overhead=power_overhead)


# At first I wanted to perse json directly to PodHost and AppInfo, but then I found that it is complicated, and SimpleNamespace is much simper, so I gave up this.
@dataclass
class PodHost:
    podIP: str
    hostName: str
    hostIP: str


@dataclass
class AppInfo:
    appName: str
    svcName: str
    deployName: str
    clusterIP: str
    nodePortIP: list[str]
    svcPort: list[str]
    nodePort: list[str]
    containerPort: list[str]
    hosts: list[PodHost]
    status: str
    priority: int
    autoScheduled: bool
