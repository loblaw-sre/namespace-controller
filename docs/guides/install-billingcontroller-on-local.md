# Installing Billing Controller on local
Billing Controller is disabled by default on local environments.

## Before you start
  - You need a playground project that has the BigQuery API enabled.
    - In this playground project, you need to create a dataset and table that
    will hold the billing annotations. The naming of this dataset and table
    doesn't matter, but you will need this information to configure the
    billing controller to talk to the correct table.
  - You need a service account, along with a key, that has BigQuery admin privileges

## Installation
  1. Rename service account key to `account.json`, and place inside of deploy/overlays/local
  1. Uncomment code block labelled for `[BILLING CONTROLLER]` inside `deploy/overlays/local/kustomization.yaml` 
  1. Fill out the following information:
    - `NC_BIGQUERY_DATASET_NAME` - What was the name of the dataset you created to hold the billing annotations of your local cluster?
    - `NC_PROJECT_ID` -  What was the ID of your playground project?
    - `NC_BIGQUERY_TABLE_NAME` - What was the name of the table you created to hold the billing annotations of your local cluster?
