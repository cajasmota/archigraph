import { BadRequestException, Body, Controller, Get, Param, Patch, ParseIntPipe, Query } from '@nestjs/common';
import { RequirePage } from '../../../common/auth/decorators/auth.decorators';
import { PermissionPage } from '../../../common/auth/page/permission-page';
import { DeviceService } from '../services/device.service';
import { DeviceListOptionalQuery } from '../dto/request/device-list.query.dto';
import type { DeviceFiltersResponse } from '../dto/response/device-filters.response.dto';
import type {
  DeviceDetailResponse,
  DeviceListItemResponse,
  LoadTestTypeResponse,
  PaginatedDeviceListResponse,
} from '../dto/response/device.response.dto';
import { DeviceCountersResponse } from '../dto/response/device-counters.response.dto';
import type { DeviceForBuildingResponse } from '../dto/response/device-for-building.response.dto';
import { DeviceBulkUpdateBodyDto } from '../dto/request/device-bulk-update.body.dto';
import type { DeviceEquipmentTypeItemResponse } from '../dto/response/device-equipment-type.response.dto';

@Controller({ path: 'devices', version: '1' })
@RequirePage(PermissionPage.Devices)
export class DeviceController {
  constructor(private readonly service: DeviceService) {}

  @Get()
  list(
    @Query('building_id', ParseIntPipe) buildingId: number,
    @Query() optional: DeviceListOptionalQuery,
  ): Promise<DeviceListItemResponse[] | PaginatedDeviceListResponse> {
    return this.service.list({ building_id: buildingId, group_id: optional.group_id, limit: optional.limit, offset: optional.offset });
  }

  @Get('counters')
  counters(@Query('group_id') groupIdRaw: string, @Query('building_id') buildingIdRaw?: string): Promise<DeviceCountersResponse> {
    if (!groupIdRaw) {
      throw new BadRequestException('group_id is required.');
    }
    const groupId = parseInt(groupIdRaw, 10);
    if (isNaN(groupId)) {
      throw new BadRequestException('group_id must be a valid integer.');
    }
    const buildingId = buildingIdRaw !== undefined ? parseInt(buildingIdRaw, 10) : undefined;
    if (buildingIdRaw !== undefined && isNaN(buildingId!)) {
      throw new BadRequestException('building_id must be a valid integer.');
    }
    return this.service.getCounters(groupId, buildingId);
  }

  @Get('filters')
  filters(@Query('group_id', ParseIntPipe) groupId: number, @Query('building_id', ParseIntPipe) buildingId: number): Promise<DeviceFiltersResponse> {
    return this.service.getFilters({ groupId, buildingId });
  }

  @Get('for_building')
  forBuilding(@Query('building_id') buildingIdRaw: string, @Query('group_id') groupIdRaw: string): Promise<DeviceForBuildingResponse> {
    if (!buildingIdRaw) {
      throw new BadRequestException('building_id is required');
    }
    if (!groupIdRaw) {
      throw new BadRequestException('group_id is required');
    }
    const buildingId = parseInt(buildingIdRaw, 10);
    const groupId = parseInt(groupIdRaw, 10);
    if (isNaN(buildingId) || isNaN(groupId)) {
      throw new BadRequestException('building_id and group_id must be integers');
    }
    return this.service.forBuilding(buildingId, groupId);
  }

  @Get('load_test_type')
  loadTestType(): LoadTestTypeResponse {
    return this.service.getLoadTestTypes();
  }

  @Get('equipment_type')
  equipmentType(): Promise<DeviceEquipmentTypeItemResponse[]> {
    return this.service.getEquipmentTypes();
  }

  @Patch('bulk')
  bulk(@Body() body: DeviceBulkUpdateBodyDto): Promise<{ success: boolean; message: string }> {
    return this.service.bulkUpdate(body.building_id, body.devices).then(() => ({ success: true, message: 'Device(s) have been updated.' }));
  }

  @Get(':deviceId')
  retrieve(@Param('deviceId', ParseIntPipe) deviceId: number, @Query('group_id') groupIdRaw?: string): Promise<DeviceDetailResponse> {
    const groupId = groupIdRaw !== undefined ? parseInt(groupIdRaw, 10) : undefined;
    return this.service.findById(deviceId, groupId);
  }
}
