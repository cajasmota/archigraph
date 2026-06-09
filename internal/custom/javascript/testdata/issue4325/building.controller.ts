import { Body, Controller, Delete, Get, HttpCode, HttpStatus, Param, ParseIntPipe, Post, Put, Query, Request } from '@nestjs/common';
import { RequirePage } from '../../../common/auth/decorators/auth.decorators';
import { PermissionPage } from '../../../common/auth/page/permission-page';
import { BuildingService } from '../services/building.service';
import { BuildingListOptionalQuery } from '../dto/request/building-list.query.dto';
import { BuildingActiveQuery } from '../dto/request/building-active.query.dto';
import { BuildingContractCountersQuery } from '../dto/request/building-contract-counters.query.dto';
import { BuildingNoticesQuery } from '../dto/request/building-notices.query.dto';
import { BuildingContractsQuery } from '../dto/request/building-contracts.query.dto';
import { BuildingViolationDevicesQuery } from '../dto/request/building-violation-devices.query.dto';
import { BuildingDevicesQuery } from '../dto/request/building-devices.query.dto';
import { BuildingInspectionDevicesQuery } from '../dto/request/building-inspection-devices.query.dto';
import { BuildingInspectionDevicesFiltersQuery } from '../dto/request/building-inspection-devices-filters.query.dto';
import { BuildingMeListQuery } from '../dto/request/building-me-list.query.dto';
import { BuildingMeFiltersQuery } from '../dto/request/building-me-filters.query.dto';
import { BuildingMeUpdateBody } from '../dto/request/building-me-update.body.dto';
import { BuildingMeDeleteBody } from '../dto/request/building-me-delete.body.dto';
import { BuildingMeMergeBody } from '../dto/request/building-me-merge.body.dto';
import { BuildingNoteCreateBody } from '../dto/request/building-note-create.body.dto';
import { BuildingNoticesBulkQuery } from '../dto/request/building-notices-bulk.query.dto';
import { BuildingRefreshBody } from '../dto/request/building-refresh.body.dto';
import { BuildingDobUpsertBody } from '../dto/request/building-dob-upsert.body.dto';
import { BuildingDetailResponse, BuildingLiteResponse, BuildingResponse, PaginatedBuildingListResponse } from '../dto/response/building.response.dto';
import { BuildingNoteResponse } from '../dto/response/building-notes.response.dto';
import { BuildingContractCountersResponse } from '../dto/response/building-contract-counters.response.dto';
import { BuildingContractFiltersResponse } from '../dto/response/building-contract-filters.response.dto';
import { BuildingNoticesResponse } from '../dto/response/building-notices.response.dto';
import { PaginatedBuildingContractsResponse } from '../dto/response/building-contracts.response.dto';
import { BuildingViolationDevicesResponse } from '../dto/response/building-violation-devices.response.dto';
import { BuildingContractClientsResponse } from '../dto/response/building-contract-clients.response.dto';
import { BuildingDevicesResponse } from '../dto/response/building-devices.response.dto';
import { BuildingInspectionDevicesResponse } from '../dto/response/building-inspection-devices.response.dto';
import {
  BuildingMeResponse,
  BuildingMeFiltersResponse,
  BuildingMeUpdateResponse,
  BuildingMeDeleteResponse,
  BuildingMeMergeResponse,
} from '../dto/response/building-me.response.dto';
import {
  BuildingDobUpsertResponse,
  BuildingRefreshResponse,
  BuildingNoteCreateResponse,
  BuildingNoteDeleteResponse,
} from '../dto/response/building-dob.response.dto';

@Controller({ path: 'buildings', version: '1' })
@RequirePage(PermissionPage.Buildings)
export class BuildingController {
  constructor(private readonly service: BuildingService) {}

  @Get('lite')
  listLite(): Promise<BuildingLiteResponse[]> {
    return this.service.lite();
  }

  @Get('active')
  listActive(@Query() query: BuildingActiveQuery): Promise<BuildingResponse[]> {
    return this.service.active({ groupIds: query.group_id });
  }

  @Get('contract-counters')
  @RequirePage(PermissionPage.ContractProposals)
  contractCounters(@Query() query: BuildingContractCountersQuery): Promise<BuildingContractCountersResponse> {
    return this.service.contractCounters(query.building_id, query.group_id);
  }

  @Get('contract-filters')
  @RequirePage(PermissionPage.ContractProposals)
  contractFilters(@Query() query: BuildingContractCountersQuery): Promise<BuildingContractFiltersResponse> {
    return this.service.contractFilters(query.building_id, query.group_id);
  }

  @Get('inspection-devices')
  inspectionDevices(@Query() query: BuildingInspectionDevicesQuery): Promise<BuildingInspectionDevicesResponse> {
    return this.service.inspectionDevices(query);
  }

  @Get('inspection-devices/filters')
  inspectionDevicesFilters(@Query() query: BuildingInspectionDevicesFiltersQuery): Promise<Record<string, unknown>> {
    return this.service.inspectionDevicesFilters(query);
  }

  @Get('inspections/maintenance-evaluations')
  maintenanceEvaluations(@Query() query: BuildingMeListQuery): Promise<BuildingMeResponse> {
    return this.service.maintenanceEvaluations(query);
  }

  @Get('inspections/maintenance-evaluations/filters')
  maintenanceEvaluationsFilters(@Query() query: BuildingMeFiltersQuery): Promise<BuildingMeFiltersResponse> {
    return this.service.maintenanceEvaluationsFilters(query);
  }

  @Put('inspections/me-manage')
  @HttpCode(HttpStatus.OK)
  updateMaintenanceEvaluations(@Body() body: BuildingMeUpdateBody): Promise<BuildingMeUpdateResponse> {
    return this.service.updateMaintenanceEvaluations(body);
  }

  @Delete('inspections/me-manage/delete')
  @HttpCode(HttpStatus.OK)
  deleteMaintenanceEvaluations(@Body() body: BuildingMeDeleteBody): Promise<BuildingMeDeleteResponse> {
    return this.service.deleteMaintenanceEvaluations(body);
  }

  @Post('inspections/me-manage/merge')
  @HttpCode(HttpStatus.OK)
  mergeMaintenanceEvaluations(@Body() body: BuildingMeMergeBody): Promise<BuildingMeMergeResponse> {
    return this.service.mergeMaintenanceEvaluations(body);
  }

  @Get('notices')
  buildingNoticesBulk(@Query() query: BuildingNoticesBulkQuery): Promise<Record<string, unknown>> {
    return this.service.buildingNoticesBulk(query.group_id, query.ids);
  }

  @Post('notes/create')
  @HttpCode(HttpStatus.OK)
  createNote(@Body() body: BuildingNoteCreateBody, @Request() req: Record<string, unknown>): Promise<BuildingNoteCreateResponse> {
    const principal = req['principal'] as { sub?: string } | undefined;
    return this.service.createNote(body, principal?.sub ?? '');
  }

  @Post('refresh')
  @HttpCode(HttpStatus.OK)
  refresh(@Body() body: BuildingRefreshBody): Promise<BuildingRefreshResponse> {
    return Promise.resolve(this.service.refreshBuildings(body));
  }

  @Post('dob')
  @HttpCode(HttpStatus.OK)
  dobUpsert(@Body() body: BuildingDobUpsertBody): Promise<BuildingDobUpsertResponse> {
    return Promise.resolve(this.service.dobUpsert(body));
  }

  @Get('violation-devices')
  violationDevices(@Query() query: BuildingViolationDevicesQuery): Promise<BuildingViolationDevicesResponse> {
    const limit = query.limit ?? 20;
    const offset = query.offset ?? 0;
    return this.service.violationDevices(query.building_id, limit, offset);
  }

  @Get()
  list(@Query() query: BuildingListOptionalQuery): Promise<BuildingResponse[] | PaginatedBuildingListResponse> {
    return this.service.list({ group_id: query.group_id, limit: query.limit, offset: query.offset });
  }

  @Get(':buildingId/contract-clients')
  @RequirePage(PermissionPage.ContractProposals)
  buildingContractClients(
    @Param('buildingId', ParseIntPipe) buildingId: number,
    @Query('limit') limit?: string,
    @Query('offset') offset?: string,
  ): Promise<BuildingContractClientsResponse> {
    const parsedLimit = limit !== undefined ? parseInt(limit, 10) : undefined;
    const parsedOffset = offset !== undefined ? parseInt(offset, 10) : undefined;
    return this.service.buildingContractClients(buildingId, parsedLimit, parsedOffset);
  }

  @Get(':buildingId/devices')
  buildingDevices(@Param('buildingId', ParseIntPipe) buildingId: number, @Query() query: BuildingDevicesQuery): Promise<BuildingDevicesResponse> {
    return this.service.buildingDevices(buildingId, query);
  }

  @Get(':buildingId/notes')
  buildingNotes(@Param('buildingId', ParseIntPipe) buildingId: number): Promise<BuildingNoteResponse[]> {
    return this.service.buildingNotes(buildingId);
  }

  @Delete(':noteId/notes/delete')
  @HttpCode(HttpStatus.OK)
  deleteNote(@Param('noteId', ParseIntPipe) noteId: number): Promise<BuildingNoteDeleteResponse> {
    return this.service.deleteNote(noteId);
  }

  @Get(':buildingId/contracts')
  @RequirePage(PermissionPage.ContractProposals)
  buildingContracts(
    @Param('buildingId', ParseIntPipe) buildingId: number,
    @Query() query: BuildingContractsQuery,
  ): Promise<PaginatedBuildingContractsResponse> {
    return this.service.buildingContracts(buildingId, query);
  }

  @Get(':buildingId/notices')
  notices(@Param('buildingId', ParseIntPipe) buildingId: number, @Query() query: BuildingNoticesQuery): Promise<BuildingNoticesResponse> {
    return this.service.buildingNotices(buildingId, query.group_id);
  }

  @Get(':buildingId/dob')
  buildingDob(@Param('buildingId', ParseIntPipe) buildingId: number): Promise<Record<string, unknown>> {
    return this.service.getBuildingDob(buildingId);
  }

  @Get(':buildingId/mass')
  buildingMass(@Param('buildingId') buildingId: string): Promise<Record<string, unknown>> {
    return this.service.getBuildingMass(buildingId);
  }

  @Get(':buildingId')
  retrieve(@Param('buildingId', ParseIntPipe) buildingId: number): Promise<BuildingDetailResponse> {
    return this.service.findById(buildingId);
  }
}
