import csv

import data_types


def write_csv(csv_file_name: str, results: list[data_types.ResultData]):
    with open(csv_file_name, 'w') as csv_file:
        writer = csv.writer(csv_file, delimiter=",")
        writer.writerow([
            "app_name", "priority", "resp_time", "resp_time_in_clouds",
            "pri_wei_resp_time", "pri_wei_resp_time_in_clouds",
            "temperature", "performance_loss", "power_overhead"
        ])

        for i, result in enumerate(results):
            writer.writerow([
                result.app_name, result.priority, result.resp_time,
                result.resp_time_in_clouds, result.pri_wei_resp_time,
                result.pri_wei_resp_time_in_clouds,
                result.temperature, result.performance_loss, result.power_overhead
            ])


def read_csv(csv_file_name: str) -> list[data_types.ResultData]:
    results: list[data_types.ResultData] = []

    with open(csv_file_name, 'r') as csv_file:
        csv_reader = csv.reader(csv_file, delimiter=",")

        header = next(csv_reader)  # skip the first row of the csv file.
        # Check if old format (without temperature fields) or new format
        has_temperature = len(header) >= 9
        
        for row in csv_reader:
            if has_temperature and len(row) >= 9:
                # New format with temperature and losses
                results.append(
                    data_types.ResultData(app_name=row[0],
                                          priority=int(row[1]),
                                          resp_time=float(row[2]),
                                          resp_time_in_clouds=float(row[3]),
                                          pri_wei_resp_time=float(row[4]),
                                          pri_wei_resp_time_in_clouds=float(row[5]),
                                          temperature=float(row[6]) if len(row) > 6 else 20.0,
                                          performance_loss=float(row[7]) if len(row) > 7 else 0.0,
                                          power_overhead=float(row[8]) if len(row) > 8 else 0.0))
            else:
                # Old format - use defaults for temperature fields
                results.append(
                    data_types.ResultData(app_name=row[0],
                                          priority=int(row[1]),
                                          resp_time=float(row[2]),
                                          resp_time_in_clouds=float(row[3]),
                                          pri_wei_resp_time=float(row[4]),
                                          pri_wei_resp_time_in_clouds=float(row[5]),
                                          temperature=20.0,  # default
                                          performance_loss=0.0,  # default
                                          power_overhead=0.0))  # default

    return results
