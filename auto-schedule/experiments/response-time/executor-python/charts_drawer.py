import glob
import json
import numpy as np
import matplotlib.pyplot as plt

import data_types
import csv_operation

REPEAT_COUNT = 60  # We repeat the experiments for REPEAT_COUNT times.
DEVICE_COUNT = 1  # we use DEVICE_COUNT devices to send requests.
APP_COUNT = 100  # In every repeat, we deploy APP_COUNT applications.
REQ_COUNT_PER_APP = 10  # In every repeat, on every device, we access every application REQ_COUNT_PER_APP times.
ALGO_NAMES = [
    "BERand", "Amaga", "Ampga", "Diktyoga", "Mcssga"
]  # the names of all algorithms to be evaluated in this experiment
OUR_ALGO_NAME = "Mcssga"

#  the minimum and maximum possible priorities
MIN_PRI = 1
MAX_PRI = 10

ATTR_TO_METRIC: dict[str, str] = {
    "resp_time":
    "Response time (ms)",
    "resp_time_in_clouds":
    "Response time in clouds (ms)",
    "pri_wei_resp_time":
    "Priority-weighted response time (ms)",
    "pri_wei_resp_time_in_clouds":
    "Priority-weighted response time in clouds (ms)",
    "temperature":
    "Temperature (°C)",
    "performance_loss":
    "Performance loss (fraction)",
    "power_overhead":
    "Power overhead (%)",
}

NON_PRI_ATTR_TO_METRIC: dict[str, str] = {
    "resp_time": "Response time (ms)",
    "resp_time_in_clouds": "Response time in clouds (ms)",
    "temperature": "Temperature (°C)",
    "performance_loss": "Performance loss (fraction)",
    "power_overhead": "Power overhead (%)",
}


# calculate the values used to complement the rejected applications.
def calc_values_for_rejected(
    existing_data: dict[str, list[data_types.ResultData]]
) -> data_types.ResultData:

    # initialization
    data_for_rej = data_types.ResultData(app_name="rej",
                                         priority=1,
                                         resp_time=0,
                                         resp_time_in_clouds=0,
                                         pri_wei_resp_time=0,
                                         pri_wei_resp_time_in_clouds=0,
                                         temperature=20.0,
                                         performance_loss=0.0,
                                         power_overhead=0.0)

    # set the values in data_for_rej as the largest value of every metric among all algorithms
    for algo_name, algo_data in existing_data.items():
        for _, one_data in enumerate(algo_data):
            if one_data.resp_time > data_for_rej.resp_time:
                data_for_rej.resp_time = one_data.resp_time

            if one_data.resp_time_in_clouds > data_for_rej.resp_time_in_clouds:
                data_for_rej.resp_time_in_clouds = one_data.resp_time_in_clouds

            if one_data.pri_wei_resp_time > data_for_rej.pri_wei_resp_time:
                data_for_rej.pri_wei_resp_time = one_data.pri_wei_resp_time

            if one_data.pri_wei_resp_time_in_clouds > data_for_rej.pri_wei_resp_time_in_clouds:
                data_for_rej.pri_wei_resp_time_in_clouds = one_data.pri_wei_resp_time_in_clouds

            # TODO: Perhaps, for the priority-weighted metrics, we may calculate them with their own priorities instead of directly using the highest priority-weighted values, to show that we only reject the low-proirity applicatioins. However, this may not be necessary, because we already have another metric: priority-weighted acceptance rate.

    # to make a difference between the rejected applicatioins and accepted ones, we enlarge the values in data_for_rej 1.1 times.
    enlarge_rate = 1.1
    data_for_rej.resp_time *= enlarge_rate
    data_for_rej.resp_time_in_clouds *= enlarge_rate
    data_for_rej.pri_wei_resp_time *= enlarge_rate
    data_for_rej.pri_wei_resp_time_in_clouds *= enlarge_rate

    return data_for_rej


# get one metric for making CDF charts
def make_cdf_data(all_data: dict[str, list[data_types.ResultData]],
                  metric_attr_name: str) -> dict[str, list[float]]:

    metric_data: dict[str, list[float]] = dict()

    for algo_name, algo_data in all_data.items():
        metric_data[algo_name] = []
        for _, one_data in enumerate(algo_data):
            metric_data[algo_name].append(getattr(
                one_data,
                metric_attr_name))  # get the attribute by the attr name

    return metric_data


def draw_cdf(cdf_data: dict[str, list[float]],
             metric_name: str,
             mark_every: int,
             title: str = ""):
    markers = ["*", "v", "+", "x", "d", "1"]
    marker_idx = 0

    plt.figure()

    # # trigger core fonts for PDF backend
    # plt.rcParams["pdf.use14corefonts"] = True
    # # trigger core fonts for PS backend
    # plt.rcParams["ps.useafm"] = True

    plt.rcParams["font.family"] = "Times New Roman"
    plt.rcParams['font.size'] = 15

    for algo_name, algo_data in cdf_data.items():
        sorted_data = np.sort(algo_data)
        cumulative_prob = np.arange(1, len(algo_data) + 1) / len(algo_data)
        plt.plot(sorted_data,
                 cumulative_prob,
                 marker=markers[marker_idx],
                 markersize=6,
                 markevery=mark_every,
                 label=algo_name)
        marker_idx += 1
        if marker_idx >= len(markers):
            marker_idx = 0
    plt.title('Cumulative Distribution Function (CDF)')
    if len(title) > 0:
        plt.title(title)
    plt.xlabel(metric_name)
    plt.ylabel('Cumulative Probability')
    plt.grid(True)
    plt.legend()

    plt.show()


# draw CDF charts for every metric
def draw_cdf_every_metric(data_to_draw: dict[str, list[data_types.ResultData]],
                          mark_every: int,
                          title: str = ""):
    for attr_name, metric_name in ATTR_TO_METRIC.items():
        metric_data = make_cdf_data(data_to_draw, attr_name)
        draw_cdf(metric_data, metric_name, mark_every, title)


# draw CDF charts for metrics not weighted by priorities
def draw_cdf_non_pri_metric(data_to_draw: dict[str,
                                               list[data_types.ResultData]],
                            mark_every: int,
                            title: str = ""):
    for attr_name, metric_name in NON_PRI_ATTR_TO_METRIC.items():
        metric_data = make_cdf_data(data_to_draw, attr_name)
        draw_cdf(metric_data, metric_name, mark_every, title)


# filter the data about the applications with the specified priority
def filter_data_with_priority(
        all_data: dict[str, list[data_types.ResultData]],
        priority: int) -> dict[str, list[data_types.ResultData]]:
    out_data: dict[str, list[data_types.ResultData]] = dict()
    for algo_name, algo_data in all_data.items():
        out_data[algo_name] = []
        for _, one_data in enumerate(algo_data):
            if one_data.priority == priority:
                out_data[algo_name].append(one_data)
    return out_data


# filter the applications accepted by all algorithms
def filter_app_data_all_accepted(
    data_this_repeat: dict[str, dict[str, list[data_types.ResultData]]],
    app_name_to_pri: dict[str, int], algo_names_to_compare: list[str]
) -> dict[str, list[data_types.ResultData]]:

    all_accepted_app_data: dict[str, list[data_types.ResultData]] = dict()
    for _, algo_name in enumerate(
            algo_names_to_compare):  # initialize the dict
        all_accepted_app_data[algo_name] = []

    for app_name, _ in app_name_to_pri.items():
        rejected = False
        for _, algo_name in enumerate(algo_names_to_compare):
            if len(data_this_repeat[algo_name][app_name]) == 0:
                rejected = True
        if not rejected:
            for _, algo_name in enumerate(algo_names_to_compare):
                all_accepted_app_data[algo_name].extend(
                    data_this_repeat[algo_name][app_name])

    return all_accepted_app_data


# draw charts for one repeat
def draw_cdf_one_repeat(
    data_this_repeat: dict[str, dict[str, list[data_types.ResultData]]],
    app_name_to_pri: dict[str, int], repeat_idx: int
) -> dict[str, dict[str, dict[str, list[data_types.ResultData]]]]:

    # return the algorithm names and the data to merge the data and draw them in one chart
    algos_to_both_acc_data: dict[str, dict[str, dict[
        str, list[data_types.ResultData]]]] = dict()
    for i, _ in enumerate(ALGO_NAMES):
        algos_to_both_acc_data[ALGO_NAMES[i]] = dict()

    # compare every 2 algorithms
    for i, _ in enumerate(ALGO_NAMES):
        for j in range(i + 1, len(ALGO_NAMES)):
            algos_to_cmp = [ALGO_NAMES[i], ALGO_NAMES[j]]
            # only draw charts for the applications accepted by both of the selected algorithms
            app_data_all_accepted = filter_app_data_all_accepted(
                data_this_repeat, app_name_to_pri, algos_to_cmp)
            # # draw
            # draw_cdf_every_metric(
            #     app_data_all_accepted, 3,
            #     "Repeat {}. Apps accepted by both {} and {}".format(
            #         repeat_idx, ALGO_NAMES[i], ALGO_NAMES[j]))
            # save data
            algos_to_both_acc_data[ALGO_NAMES[i]][
                ALGO_NAMES[j]] = app_data_all_accepted

    return algos_to_both_acc_data


def main():
    all_data: dict[str, list[data_types.ResultData]] = dict()
    for _, algo_name in enumerate(ALGO_NAMES):  # initialize the dict
        all_data[algo_name] = []

    # initializa this variable to save data to merge the applications accepted by both of every 2 algorithms
    all_repeats_data_acc_both_algos: dict[str, dict[str, dict[
        str, list[data_types.ResultData]]]] = dict()
    for i, _ in enumerate(ALGO_NAMES):
        all_repeats_data_acc_both_algos[ALGO_NAMES[i]] = dict()
        for j in range(i + 1, len(ALGO_NAMES)):
            all_repeats_data_acc_both_algos[ALGO_NAMES[i]][
                ALGO_NAMES[j]] = dict()
            all_repeats_data_acc_both_algos[ALGO_NAMES[i]][ALGO_NAMES[j]][
                ALGO_NAMES[i]] = []
            all_repeats_data_acc_both_algos[ALGO_NAMES[i]][ALGO_NAMES[j]][
                ALGO_NAMES[j]] = []

    # load all data
    for i in range(REPEAT_COUNT):
        # in a repeat, the apps are the same for all algorithms
        app_name_to_pri: dict[str, int] = dict()

        # from the app deploy request json to read all applications including rejected ones
        with open("data/repeat{}/request_applications.json".format(
                i + 1)) as json_file:
            app_reqs = json.load(json_file)
            for _, app_req in enumerate(app_reqs):
                app_name_to_pri[app_req["name"]] = app_req["priority"]

        # to draw the chart for this repeat
        all_data_this_repeat: dict[str,
                                   dict[str,
                                        list[data_types.ResultData]]] = dict()
        for _, algo_name in enumerate(ALGO_NAMES):  # initialize the dict
            all_data_this_repeat[algo_name] = dict()
            for app_name, _ in app_name_to_pri.items():
                all_data_this_repeat[algo_name][app_name] = []

        # read the data from csv files
        for algo_name in ALGO_NAMES:
            # use glob to get the paths of all csv files in this folder
            csv_files_pattern = "data/repeat{}/{}/*.csv".format(
                i + 1, algo_name)
            csv_file_names = glob.glob(csv_files_pattern)
            # read every csv files in this folder
            for _, file_name in enumerate(csv_file_names):
                data_in_file = csv_operation.read_csv(file_name)
                all_data[algo_name].extend(data_in_file)

                # to draw the chart for this repeat
                for _, one_item in enumerate(data_in_file):
                    all_data_this_repeat[algo_name][one_item.app_name].append(
                        one_item)

                # For this file, complement the data for rejected applications
                app_name_to_pri_this_file: dict[str, int] = dict()
                for name, pri in app_name_to_pri.items():
                    app_name_to_pri_this_file[name] = pri

                for _, data_item in enumerate(data_in_file):
                    del app_name_to_pri_this_file[data_item.app_name]

                if len(data_in_file) + len(
                        app_name_to_pri_this_file) != APP_COUNT:
                    print("total app count:", APP_COUNT)
                    print("accepted app count:", len(data_in_file))
                    print("rejected app count:",
                          len(app_name_to_pri_this_file))
                    raise Exception(
                        "accepted app count, rejected app count, and total app count are not mapped."
                    )

                for name, pri in app_name_to_pri_this_file.items():
                    all_data[algo_name].append(
                        data_types.ResultData(app_name=name,
                                              priority=pri,
                                              resp_time=-1,
                                              resp_time_in_clouds=-1,
                                              pri_wei_resp_time=-1,
                                              pri_wei_resp_time_in_clouds=-1,
                                              temperature=20.0,
                                              performance_loss=0.0,
                                              power_overhead=0.0))

        # to draw the chart for this repeat
        print("draw cdf charts for repeat {}".format(i + 1))
        this_repeat_algos_to_both_acc_data = draw_cdf_one_repeat(
            all_data_this_repeat, app_name_to_pri, i + 1)

        # merge data of all repeats
        for j, _ in enumerate(ALGO_NAMES):
            for k in range(j + 1, len(ALGO_NAMES)):
                all_repeats_data_acc_both_algos[ALGO_NAMES[j]][ALGO_NAMES[k]][
                    ALGO_NAMES[j]].extend(this_repeat_algos_to_both_acc_data[
                        ALGO_NAMES[j]][ALGO_NAMES[k]][ALGO_NAMES[j]])
                all_repeats_data_acc_both_algos[ALGO_NAMES[j]][ALGO_NAMES[k]][
                    ALGO_NAMES[k]].extend(this_repeat_algos_to_both_acc_data[
                        ALGO_NAMES[j]][ALGO_NAMES[k]][ALGO_NAMES[k]])

    print(
        "draw charts for the applications accepted by both of every 2 algorithms in all repeats"
    )
    for i, _ in enumerate(ALGO_NAMES):
        for j in range(i + 1, len(ALGO_NAMES)):
            if ALGO_NAMES[i] != OUR_ALGO_NAME and ALGO_NAMES[
                    j] != OUR_ALGO_NAME:
                continue
            # draw
            draw_cdf_every_metric(
                all_repeats_data_acc_both_algos[ALGO_NAMES[i]][ALGO_NAMES[j]],
                3, "Applications accepted by both {} and {}".format(
                    ALGO_NAMES[i], ALGO_NAMES[j]))

    # ----------------------------
    # complement the data for rejected applications
    # comp_count_per_algo = REPEAT_COUNT * DEVICE_COUNT * APP_COUNT * REQ_COUNT_PER_APP
    data_for_rej_apps = calc_values_for_rejected(all_data)

    # # deprecated old code, complementing data without considering the priorities of rejected applications.
    # print(f"Every algorithm should have {comp_count_per_algo} items of data.")
    # for _, algo_name in enumerate(ALGO_NAMES):
    #     algo_data = all_data[algo_name]
    #     num_to_comp = comp_count_per_algo - len(algo_data)

    #     print(
    #         f"Algorithm: {algo_name} has {len(algo_data)} items, so complement {num_to_comp} items to it."
    #     )

    #     all_data[algo_name].extend(
    #         [data_for_rej_apps for i in range(num_to_comp)])

    # Even for the rejected applications, we complement their priority-weighted metrics according to their priorities
    for _, algo_name in enumerate(ALGO_NAMES):
        for idx, _ in enumerate(all_data[algo_name]):
            if all_data[algo_name][idx].resp_time == -1:
                all_data[algo_name][
                    idx].resp_time = data_for_rej_apps.resp_time
                all_data[algo_name][
                    idx].pri_wei_resp_time = data_for_rej_apps.pri_wei_resp_time
                # all_data[algo_name][idx].pri_wei_resp_time = float(
                #     all_data[algo_name]
                #     [idx].priority) * all_data[algo_name][idx].resp_time

                all_data[algo_name][
                    idx].resp_time_in_clouds = data_for_rej_apps.resp_time_in_clouds
                all_data[algo_name][
                    idx].pri_wei_resp_time_in_clouds = data_for_rej_apps.pri_wei_resp_time_in_clouds
                # all_data[algo_name][
                #     idx].pri_wei_resp_time_in_clouds = all_data[algo_name][
                #         idx].resp_time_in_clouds * float(
                #             all_data[algo_name][idx].priority)

    # -------------------------

    # # print the complemented results
    # for algo_name, algo_data in all_data.items():
    #     print(f"Algorithm: {algo_name} has {len(algo_data)} items.")
    #     for _, item in enumerate(algo_data):
    #         print(item)
    #     print()
    #     print()

    print("draw cdf charts for all data")
    draw_cdf_every_metric(all_data, 20, "Applications with all priorities")

    # draw cdf charts for every-priority data
    for pri in range(MIN_PRI, MAX_PRI + 1):
        this_pri_data = filter_data_with_priority(all_data, pri)
        print(
            "draw cdf charts for the data about applications with priority {}".
            format(pri))
        draw_cdf_non_pri_metric(this_pri_data, 1,
                                "Applications with priority {}".format(pri))


if __name__ == "__main__":
    main()
