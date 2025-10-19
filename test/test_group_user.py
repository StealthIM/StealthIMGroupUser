import pytest
import asyncio
import pytest_asyncio
import time
from grpclib.client import Channel
import groupuser_pb2
from groupuser_grpc import StealthIMGroupUserStub
import user_pb2
from user_grpc import StealthIMUserStub
import random

username_perfix = str(random.randint(100000, 999999))


async def register_user(stub: StealthIMUserStub, username: str, password="password"):
    response = await stub.Register(user_pb2.RegisterRequest(
        username=username,
        password=password,
        nickname=f"{username}_nick"
    ))
    # 根据proto定义，RegisterResponse中只有result字段
    # 我们使用result.code == 0表示成功
    assert response.result.code == 800


async def login_user(stub: StealthIMUserStub, username):
    response = await stub.GetUIDByUsername(user_pb2.GetUIDByUsernameRequest(
        username=username
    ))
    assert response.result.code == 800
    assert response.user_id != 0
    return response.user_id


guser_lst = []


@pytest_asyncio.fixture()
async def user_channel():
    # 添加重试机制
    for _ in range(3):
        try:
            async with Channel("127.0.0.1", 50055) as channel:
                yield channel
                return
        except ConnectionRefusedError:
            time.sleep(1)  # 等待1秒后重试
    raise ConnectionError(
        "Failed to connect to User service after 3 attempts")


@pytest_asyncio.fixture()
async def group_user_channel():
    # 添加重试机制
    for _ in range(3):
        try:
            async with Channel("127.0.0.1", 50058) as channel:
                yield channel
                return
        except ConnectionRefusedError:
            time.sleep(1)  # 等待1秒后重试
    raise ConnectionError(
        "Failed to connect to GroupUser service after 3 attempts")


@pytest_asyncio.fixture()
async def user_stub(user_channel):
    return StealthIMUserStub(user_channel)


@pytest_asyncio.fixture()
async def group_user_stub(group_user_channel):
    return StealthIMGroupUserStub(group_user_channel)

user_created = False


@pytest_asyncio.fixture()
async def user_lst(user_stub: StealthIMUserStub):
    global user_created, guser_lst
    if not user_created:
        for attempt in range(5):
            try:
                await register_user(user_stub, username_perfix+"_acc1")
                await register_user(user_stub, username_perfix+"_acc2")
                await register_user(user_stub, username_perfix+"_acc3")
                await register_user(user_stub, username_perfix+"_acc4")
                guser_lst = []
                guser_lst.append(await login_user(
                    user_stub, username_perfix+"_acc1"))
                guser_lst.append(await login_user(
                    user_stub, username_perfix+"_acc2"))
                guser_lst.append(await login_user(
                    user_stub, username_perfix+"_acc3"))
                guser_lst.append(await login_user(
                    user_stub, username_perfix+"_acc4"))
                user_created = True
                return guser_lst
            except Exception as e:
                if attempt == 4:  # 最后一次尝试后仍失败则抛出异常
                    raise
                await asyncio.sleep(1)  # 等待1秒后重试
    else:
        return guser_lst


@pytest.mark.asyncio
async def test_ping(group_user_stub: StealthIMGroupUserStub):
    # 添加重试机制，使用异步sleep
    for attempt in range(5):
        try:
            response = await group_user_stub.Ping(groupuser_pb2.PingRequest())
            assert isinstance(response, groupuser_pb2.Pong)
            return
        except Exception as e:
            if attempt == 4:  # 最后一次尝试后仍失败则抛出异常
                raise
            await asyncio.sleep(1)  # 使用异步等待1秒


@pytest.mark.asyncio
async def test_group_lifecycle(group_user_stub: StealthIMGroupUserStub, user_lst: list):

    # 创建群组
    create_resp = await group_user_stub.CreateGroup(groupuser_pb2.CreateGroupRequest(
        name="grp1",
        uid=user_lst[0]
    ))
    assert create_resp.result.code == 800
    group_id = create_resp.group_id

    # 验证群组信息
    pinfo_resp = await group_user_stub.GetGroupPublicInfo(groupuser_pb2.GetGroupPublicInfoRequest(
        group_id=group_id,
    ))
    assert pinfo_resp.result.code == 800
    assert pinfo_resp.name == "grp1"

    info_resp = await group_user_stub.GetGroupInfo(groupuser_pb2.GetGroupInfoRequest(
        group_id=group_id,
        uid=user_lst[0]
    ))
    assert info_resp.result.code == 800
    assert info_resp.members[0].name == username_perfix+"_acc1"


@pytest.mark.asyncio
async def test_group_info_permission(group_user_stub: StealthIMGroupUserStub, user_lst: list):

    # 创建群组
    create_resp = await group_user_stub.CreateGroup(groupuser_pb2.CreateGroupRequest(
        name="grp2",
        uid=user_lst[0]
    ))
    assert create_resp.result.code == 800
    group_id = create_resp.group_id

    info_resp = await group_user_stub.GetGroupInfo(groupuser_pb2.GetGroupInfoRequest(
        group_id=group_id,
        uid=user_lst[1]
    ))
    assert info_resp.result.code != 800


@pytest.mark.asyncio
async def test_group_join(group_user_stub: StealthIMGroupUserStub, user_lst: list):
    # 创建群组
    create_resp = await group_user_stub.CreateGroup(groupuser_pb2.CreateGroupRequest(
        name="grp3",
        uid=user_lst[0]
    ))
    assert create_resp.result.code == 800
    group_id = create_resp.group_id

    join_resp = await group_user_stub.JoinGroup(groupuser_pb2.JoinGroupRequest(
        group_id=1145141919,
        password="",
        uid=user_lst[1]
    ))
    assert join_resp.result.code != 800

    join_resp = await group_user_stub.JoinGroup(groupuser_pb2.JoinGroupRequest(
        group_id=group_id,
        password="error_password",
        uid=user_lst[1]
    ))
    assert join_resp.result.code != 800

    join_resp = await group_user_stub.JoinGroup(groupuser_pb2.JoinGroupRequest(
        group_id=group_id,
        password="",
        uid=user_lst[1]
    ))
    assert join_resp.result.code == 800

    join_resp = await group_user_stub.JoinGroup(groupuser_pb2.JoinGroupRequest(
        group_id=group_id,
        password="",
        uid=user_lst[1]
    ))
    assert join_resp.result.code != 800

    info_resp = await group_user_stub.GetGroupInfo(groupuser_pb2.GetGroupInfoRequest(
        group_id=group_id,
        uid=user_lst[0]
    ))
    assert info_resp.result.code == 800
    assert (username_perfix+"_acc1",
            groupuser_pb2.MemberType.owner) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc2",
            groupuser_pb2.MemberType.member) in [(m.name, m.type) for m in info_resp.members]


@pytest.mark.asyncio
async def test_group_change_passwd(group_user_stub: StealthIMGroupUserStub, user_lst: list):
    # 创建群组
    create_resp = await group_user_stub.CreateGroup(groupuser_pb2.CreateGroupRequest(
        name="grp4",
        uid=user_lst[0]
    ))
    assert create_resp.result.code == 800
    group_id = create_resp.group_id

    change_resp = await group_user_stub.ChangeGroupPassword(groupuser_pb2.ChangeGroupPasswordRequest(
        group_id=group_id,
        password="right_password",
        uid=user_lst[0]
    ))
    assert change_resp.result.code == 800
    change_resp = await group_user_stub.ChangeGroupPassword(groupuser_pb2.ChangeGroupPasswordRequest(
        group_id=group_id,
        password="right_password",
        uid=user_lst[1]
    ))
    assert change_resp.result.code != 800

    join_resp = await group_user_stub.JoinGroup(groupuser_pb2.JoinGroupRequest(
        group_id=group_id,
        password="",
        uid=user_lst[1]
    ))
    assert join_resp.result.code != 800

    join_resp = await group_user_stub.JoinGroup(groupuser_pb2.JoinGroupRequest(
        group_id=group_id,
        password="right_password",
        uid=user_lst[1]
    ))
    assert join_resp.result.code == 800

    change_resp = await group_user_stub.ChangeGroupPassword(groupuser_pb2.ChangeGroupPasswordRequest(
        group_id=group_id,
        password="right_password",
        uid=user_lst[1]
    ))
    assert change_resp.result.code != 800

    info_resp = await group_user_stub.GetGroupInfo(groupuser_pb2.GetGroupInfoRequest(
        group_id=group_id,
        uid=user_lst[0]
    ))
    assert info_resp.result.code == 800
    assert (username_perfix+"_acc1",
            groupuser_pb2.MemberType.owner) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc2",
            groupuser_pb2.MemberType.member) in [(m.name, m.type) for m in info_resp.members]


@pytest.mark.asyncio
async def test_group_invite(group_user_stub: StealthIMGroupUserStub, user_lst: list):
    # 创建群组
    create_resp = await group_user_stub.CreateGroup(groupuser_pb2.CreateGroupRequest(
        name="grp5",
        uid=user_lst[0]
    ))
    assert create_resp.result.code == 800
    group_id = create_resp.group_id

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[0],
        username="fake_username"
    ))
    assert join_resp.result.code != 800

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[1],
        username=username_perfix+"_acc2"
    ))
    assert join_resp.result.code != 800

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc2"
    ))
    assert join_resp.result.code == 800

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc2"
    ))
    assert join_resp.result.code != 800

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[1],
        username=username_perfix+"_acc3"
    ))
    assert join_resp.result.code == 800

    info_resp = await group_user_stub.GetGroupInfo(groupuser_pb2.GetGroupInfoRequest(
        group_id=group_id,
        uid=user_lst[0]
    ))
    assert info_resp.result.code == 800
    assert (username_perfix+"_acc1",
            groupuser_pb2.MemberType.owner) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc2",
            groupuser_pb2.MemberType.member) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc3",
            groupuser_pb2.MemberType.member) in [(m.name, m.type) for m in info_resp.members]


@pytest.mark.asyncio
async def test_group_settype(group_user_stub: StealthIMGroupUserStub, user_lst: list):
    # 创建群组
    create_resp = await group_user_stub.CreateGroup(groupuser_pb2.CreateGroupRequest(
        name="grp6",
        uid=user_lst[0]
    ))
    assert create_resp.result.code == 800
    group_id = create_resp.group_id

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc2"
    ))
    assert join_resp.result.code == 800

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc3"
    ))
    assert join_resp.result.code == 800

    set_resp = await group_user_stub.SetUserType(groupuser_pb2.SetUserTypeRequest(
        group_id=group_id,
        uid=user_lst[0],
        username="fake_username",
        type=groupuser_pb2.MemberType.manager,
    ))
    assert set_resp.result.code != 800

    set_resp = await group_user_stub.SetUserType(groupuser_pb2.SetUserTypeRequest(
        group_id=group_id,
        uid=user_lst[1],
        username=username_perfix+"_acc2",
        type=groupuser_pb2.MemberType.manager,
    ))
    assert set_resp.result.code != 800

    set_resp = await group_user_stub.SetUserType(groupuser_pb2.SetUserTypeRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc4",
        type=groupuser_pb2.MemberType.manager,
    ))
    assert set_resp.result.code != 800
    set_resp = await group_user_stub.SetUserType(groupuser_pb2.SetUserTypeRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc2",
        type=groupuser_pb2.MemberType.manager,
    ))
    assert set_resp.result.code == 800

    await asyncio.sleep(3)

    info_resp = await group_user_stub.GetGroupInfo(groupuser_pb2.GetGroupInfoRequest(
        group_id=group_id,
        uid=user_lst[0]
    ))
    assert info_resp.result.code == 800
    assert (username_perfix+"_acc1",
            groupuser_pb2.MemberType.owner) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc2",
            groupuser_pb2.MemberType.manager) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc3",
            groupuser_pb2.MemberType.member) in [(m.name, m.type) for m in info_resp.members]


@pytest.mark.asyncio
async def test_group_changename(group_user_stub: StealthIMGroupUserStub, user_lst: list):
    # 创建群组
    create_resp = await group_user_stub.CreateGroup(groupuser_pb2.CreateGroupRequest(
        name="grp7",
        uid=user_lst[0]
    ))
    assert create_resp.result.code == 800
    group_id = create_resp.group_id

    info_resp = await group_user_stub.GetGroupPublicInfo(groupuser_pb2.GetGroupPublicInfoRequest(
        group_id=group_id
    ))
    assert info_resp.result.code == 800
    assert info_resp.name == "grp7"

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc2"
    ))
    assert join_resp.result.code == 800

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc3"
    ))
    assert join_resp.result.code == 800

    set_resp = await group_user_stub.SetUserType(groupuser_pb2.SetUserTypeRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc2",
        type=groupuser_pb2.MemberType.manager,
    ))
    assert set_resp.result.code == 800

    change_resp = await group_user_stub.ChangeGroupName(groupuser_pb2.ChangeGroupNameRequest(
        group_id=group_id,
        uid=user_lst[0],
        name="grp7_new1"
    ))
    assert change_resp.result.code == 800

    info_resp = await group_user_stub.GetGroupPublicInfo(groupuser_pb2.GetGroupPublicInfoRequest(
        group_id=group_id
    ))
    assert info_resp.result.code == 800
    assert info_resp.name == "grp7_new1"

    change_resp = await group_user_stub.ChangeGroupName(groupuser_pb2.ChangeGroupNameRequest(
        group_id=group_id,
        uid=user_lst[1],
        name="grp7_new2"
    ))
    assert change_resp.result.code == 800

    info_resp = await group_user_stub.GetGroupPublicInfo(groupuser_pb2.GetGroupPublicInfoRequest(
        group_id=group_id
    ))
    assert info_resp.result.code == 800
    assert info_resp.name == "grp7_new2"

    change_resp = await group_user_stub.ChangeGroupName(groupuser_pb2.ChangeGroupNameRequest(
        group_id=group_id,
        uid=user_lst[2],
        name="grp7_new3"
    ))
    assert change_resp.result.code != 800

    info_resp = await group_user_stub.GetGroupPublicInfo(groupuser_pb2.GetGroupPublicInfoRequest(
        group_id=group_id
    ))
    assert info_resp.result.code == 800
    assert info_resp.name == "grp7_new2"


@pytest.mark.asyncio
async def test_user_getgroups(group_user_stub: StealthIMGroupUserStub, user_lst: list):
    # 创建群组
    grps_resp = await group_user_stub.GetGroupsByUID(groupuser_pb2.GetGroupsByUIDRequest(
        uid=user_lst[3]
    ))
    assert grps_resp.result.code == 800
    assert len(grps_resp.groups) == 0

    create_resp = await group_user_stub.CreateGroup(groupuser_pb2.CreateGroupRequest(
        name="grp8_1",
        uid=user_lst[3]
    ))

    grps_resp = await group_user_stub.GetGroupsByUID(groupuser_pb2.GetGroupsByUIDRequest(
        uid=user_lst[3]
    ))
    assert grps_resp.result.code == 800
    assert len(grps_resp.groups) == 1

    assert create_resp.result.code == 800
    create_resp2 = await group_user_stub.CreateGroup(groupuser_pb2.CreateGroupRequest(
        name="grp8_2",
        uid=user_lst[1]
    ))
    assert create_resp2.result.code == 800
    group_id2 = create_resp2.group_id

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id2,
        uid=user_lst[1],
        username=username_perfix+"_acc4"
    ))
    assert join_resp.result.code == 800

    await asyncio.sleep(1)

    grps_resp = await group_user_stub.GetGroupsByUID(groupuser_pb2.GetGroupsByUIDRequest(
        uid=user_lst[3]
    ))
    assert grps_resp.result.code == 800
    assert len(grps_resp.groups) == 2


@pytest.mark.asyncio
async def test_group_kickuser(group_user_stub: StealthIMGroupUserStub, user_lst: list):
    # 创建群组
    create_resp = await group_user_stub.CreateGroup(groupuser_pb2.CreateGroupRequest(
        name="grp9",
        uid=user_lst[0]
    ))
    assert create_resp.result.code == 800
    group_id = create_resp.group_id

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc2"
    ))
    assert join_resp.result.code == 800

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc3"
    ))
    assert join_resp.result.code == 800

    set_resp = await group_user_stub.SetUserType(groupuser_pb2.SetUserTypeRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc2",
        type=groupuser_pb2.MemberType.manager,
    ))
    assert set_resp.result.code == 800

    info_resp = await group_user_stub.GetGroupInfo(groupuser_pb2.GetGroupInfoRequest(
        group_id=group_id,
        uid=user_lst[0]
    ))
    assert info_resp.result.code == 800
    assert (username_perfix+"_acc1",
            groupuser_pb2.MemberType.owner) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc2",
            groupuser_pb2.MemberType.manager) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc3",
            groupuser_pb2.MemberType.member) in [(m.name, m.type) for m in info_resp.members]

    # kick_resp = await group_user_stub.KickUser(groupuser_pb2.KickUserRequest(
    #     group_id=group_id,
    #     uid=user_lst[2],
    #     username=username_perfix+"_acc3"
    # ))
    # assert kick_resp.result.code != 800

    kick_resp = await group_user_stub.KickUser(groupuser_pb2.KickUserRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc4"
    ))
    assert kick_resp.result.code != 800

    kick_resp = await group_user_stub.KickUser(groupuser_pb2.KickUserRequest(
        group_id=group_id,
        uid=user_lst[0],
        username="fake_user"
    ))
    assert kick_resp.result.code != 800

    grps_resp = await group_user_stub.GetGroupsByUID(groupuser_pb2.GetGroupsByUIDRequest(
        uid=user_lst[2]
    ))
    assert grps_resp.result.code == 800
    x = len(grps_resp.groups)

    kick_resp = await group_user_stub.KickUser(groupuser_pb2.KickUserRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc3"
    ))
    assert kick_resp.result.code == 800

    info_resp = await group_user_stub.GetGroupInfo(groupuser_pb2.GetGroupInfoRequest(
        group_id=group_id,
        uid=user_lst[0]
    ))
    assert info_resp.result.code == 800
    assert (username_perfix+"_acc1",
            groupuser_pb2.MemberType.owner) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc2",
            groupuser_pb2.MemberType.manager) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc3",
            groupuser_pb2.MemberType.member) not in [(m.name, m.type) for m in info_resp.members]
    await asyncio.sleep(1)

    grps_resp = await group_user_stub.GetGroupsByUID(groupuser_pb2.GetGroupsByUIDRequest(
        uid=user_lst[2]
    ))
    assert grps_resp.result.code == 800
    assert len(grps_resp.groups) == x-1


@pytest.mark.asyncio
async def test_group_leave(group_user_stub: StealthIMGroupUserStub, user_lst: list):
    # 创建群组
    create_resp = await group_user_stub.CreateGroup(groupuser_pb2.CreateGroupRequest(
        name="grp10",
        uid=user_lst[0]
    ))
    assert create_resp.result.code == 800
    group_id = create_resp.group_id
    await asyncio.sleep(1)

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc2"
    ))
    assert join_resp.result.code == 800

    join_resp = await group_user_stub.InviteGroup(groupuser_pb2.InviteGroupRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc3"
    ))
    assert join_resp.result.code == 800

    set_resp = await group_user_stub.SetUserType(groupuser_pb2.SetUserTypeRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc2",
        type=groupuser_pb2.MemberType.manager,
    ))
    assert set_resp.result.code == 800

    info_resp = await group_user_stub.GetGroupInfo(groupuser_pb2.GetGroupInfoRequest(
        group_id=group_id,
        uid=user_lst[0]
    ))
    assert info_resp.result.code == 800
    assert (username_perfix+"_acc1",
            groupuser_pb2.MemberType.owner) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc2",
            groupuser_pb2.MemberType.manager) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc3",
            groupuser_pb2.MemberType.member) in [(m.name, m.type) for m in info_resp.members]

    # kick_resp = await group_user_stub.KickUser(groupuser_pb2.KickUserRequest(
    #     group_id=group_id,
    #     uid=user_lst[0],
    #     username=username_perfix+"_acc1"
    # ))
    # assert kick_resp.result.code == 800

    kick_resp = await group_user_stub.KickUser(groupuser_pb2.KickUserRequest(
        group_id=group_id,
        uid=user_lst[1],
        username=username_perfix+"_acc2"
    ))
    assert kick_resp.result.code == 800

    kick_resp = await group_user_stub.KickUser(groupuser_pb2.KickUserRequest(
        group_id=group_id,
        uid=user_lst[2],
        username=username_perfix+"_acc3"
    ))
    assert kick_resp.result.code == 800

    info_resp = await group_user_stub.GetGroupInfo(groupuser_pb2.GetGroupInfoRequest(
        group_id=group_id,
        uid=user_lst[0]
    ))
    assert info_resp.result.code == 800
    assert (username_perfix+"_acc1",
            groupuser_pb2.MemberType.owner) in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc2",
            groupuser_pb2.MemberType.manager) not in [(m.name, m.type) for m in info_resp.members]
    assert (username_perfix+"_acc3",
            groupuser_pb2.MemberType.member) not in [(m.name, m.type) for m in info_resp.members]
    kick_resp = await group_user_stub.KickUser(groupuser_pb2.KickUserRequest(
        group_id=group_id,
        uid=user_lst[0],
        username=username_perfix+"_acc1"
    ))
    assert kick_resp.result.code == 800
