/*
Copyright 2022 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package job

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
)

type HelmChartDeployJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.ZadigHelmChartDeployJobSpec
}

func (j *HelmChartDeployJob) Instantiate() error {
	j.spec = &commonmodels.ZadigHelmChartDeployJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *HelmChartDeployJob) SetPreset() error {
	j.spec = &commonmodels.ZadigHelmChartDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: j.spec.Env})
	if err != nil {
		return fmt.Errorf("env %s not exists", j.spec.Env)
	}
	renderset, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		Name:        product.Render.Name,
		ProductTmpl: product.ProductName,
		Revision:    product.Render.Revision,
	})
	if err != nil {
		return fmt.Errorf("render set %s/%v not found", product.Render.Name, product.Render.Revision)
	}
	renderChartMap := renderset.GetChartDeployRenderMap()

	deploys := []*commonmodels.DeployHelmChart{}
	productChartServiceMap := product.GetChartServiceMap()
	for _, chartSvc := range productChartServiceMap {
		renderChart := renderChartMap[chartSvc.ReleaseName]
		if renderChart == nil {
			return fmt.Errorf("render chart %s not found", chartSvc.ReleaseName)
		}
		deploy := &commonmodels.DeployHelmChart{
			ReleaseName:  chartSvc.ReleaseName,
			ChartRepo:    renderChart.ChartRepo,
			ChartName:    renderChart.ChartName,
			ChartVersion: renderChart.ChartVersion,
			ValuesYaml:   renderChart.GetOverrideYaml(),
		}
		deploys = append(deploys, deploy)
	}
	j.spec.DeployHelmCharts = deploys
	j.job.Spec = j.spec

	return nil
}

func (j *HelmChartDeployJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.ZadigHelmChartDeployJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.ZadigHelmChartDeployJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		j.spec.Env = argsSpec.Env
		j.spec.DeployHelmCharts = argsSpec.DeployHelmCharts

		j.job.Spec = j.spec
	}
	return nil
}

func (j *HelmChartDeployJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.ZadigHelmChartDeployJobSpec{}

	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec

	envName := j.spec.Env
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: j.workflow.Project, EnvName: envName})
	if err != nil {
		return resp, fmt.Errorf("env %s not exists", envName)
	}

	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	for _, deploy := range j.spec.DeployHelmCharts {
		jobTaskSpec := &commonmodels.JobTaskHelmChartDeploySpec{
			Env:                envName,
			DeployHelmChart:    deploy,
			SkipCheckRunStatus: j.spec.SkipCheckRunStatus,
			ClusterID:          product.ClusterID,
			Timeout:            timeout,
		}

		jobTask := &commonmodels.JobTask{
			Name: j.job.Name,
			Key:  j.job.Name,
			JobInfo: map[string]string{
				JobNameKey:     j.job.Name,
				"release_name": deploy.ReleaseName,
			},
			JobType: string(config.JobZadigHelmChartDeploy),
			Spec:    jobTaskSpec,
		}
		resp = append(resp, jobTask)
	}
	return resp, nil
}

func (j *HelmChartDeployJob) LintJob() error {
	j.spec = &commonmodels.ZadigHelmChartDeployJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	return nil
}
