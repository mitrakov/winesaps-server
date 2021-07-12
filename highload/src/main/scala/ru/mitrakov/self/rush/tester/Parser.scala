package ru.mitrakov.self.rush.tester

import akka.actor.Actor

/**
  * Created by mitrakov on 05.07.2017.
  */
class Parser extends Actor {
  override def receive: Receive = {
    case SingleMessage(crcId, sid, token, data) =>
      sender() ! parse(crcId, sid, token, data)
    case x: Message => sender() ! OutputMessage(x.crcId, parse(x))
    case x => println(s"[Parser] Unknown message: $x")
  }

  private def parse(crcId: Int, sid: Int, token: Int, data: List[Int]): Message = {
    data match {
      case 2 :: error :: Nil => SignIn(crcId, sid, token, 0, "", "", "", error)
      case 3 :: error :: Nil => SignOut(crcId, sid, token, error)
      case 4 :: error :: tail =>
        val (nameLst, rest0) = tail.splitAt(tail.indexOf(0))
        val name = new String(nameLst.map(_.toByte).toArray)
        val promocodeRest = rest0.tail
        val (promocodeLst, rest) = promocodeRest.splitAt(promocodeRest.indexOf(0))
        val promocode = new String(promocodeLst.map{_.toByte}.toArray)
        val character = rest(1)
        val gems = (rest(2) << 24) | (rest(3) << 16) | (rest(4) << 8) | rest(5)
        val abilityLst = rest.drop(7)
        val abilities = abilityLst.grouped(3).toList map {triple => triple.head -> (triple(1)*256 + triple(2))}
        UserInfo(crcId, sid, token, error, name, promocode, character, gems, abilities)
      case 6 :: error :: Nil => Attack(crcId, sid, token, 0, "", error)
      case 8 :: error :: Nil => Accept(crcId, sid, token, 0, error)
      case 13 :: error :: tail =>
        val abilities = tail.grouped(3).toList map {triple => (triple.head, triple(1), triple(2))}
        RangeOfProducts(crcId, sid, token, error, abilities)
      case 15 :: tail => EnemyName(crcId, sid, token, new String(tail.map{_.toByte}.toArray))
      case 16 :: tail => FullState(crcId, sid, token, tail)
      case 17 :: number :: time :: aggressor :: char1 :: char2 :: myLives :: enemyLives :: tail =>
        val fieldName = new String(tail.map{_.toByte}.toArray)
        RoundInfo(crcId, sid, token, number, time, aggressor == 1, char1, char2, myLives, enemyLives, fieldName)
      case 18 :: _ :: tail => AbilityList(crcId, sid, token, tail)
      case 19 :: error :: Nil => Move(crcId, sid, token, 0, error)
      case 20 :: error :: Nil => UseThing(crcId, sid, token, error)
      case 21 :: error :: Nil => UseSkill(crcId, sid, token, 0, error)
      case 22 :: error :: Nil => GiveUp(crcId, sid, token, error)
      case 23 :: objNum :: objID :: xy :: reset :: Nil => StateChanged(crcId, sid, token, objNum, objID, xy, reset == 1)
      case 24 :: score1 :: score2 :: Nil => ScoreChanged(crcId, sid, token, score1, score2)
      case 25 :: id :: added :: objNumber :: Nil => EffectChanged(crcId, sid, token, id, added == 1, objNumber)
      case 26 :: me :: cause :: myLives :: enemyLives :: Nil =>
        PlayerWounded(crcId, sid, token, me == 1, cause, myLives, enemyLives)
      case 27 :: me :: objectID :: Nil => ThingTaken(crcId, sid, token, me == 1, objectID)
      case 28 :: id :: objectNum :: xy :: Nil => ObjectAppended(crcId, sid, token, id, objectNum, xy)
      case 29 :: gameOver :: me :: score1 :: score2 :: Nil =>
        Finished(crcId, sid, token, gameOver == 1, me == 1, score1, score2)
      case 29 :: gameOver :: me :: score1 :: score2 :: rewardA :: rewardB :: rewardC :: rewardD :: Nil =>
        val reward = (rewardA << 24) | (rewardB << 16) | (rewardC << 8) | rewardD
        Finished2(crcId, sid, token, gameOver == 1, me == 1, score1, score2, reward)
      case 33 :: error :: fragNumber :: tail =>
        val s = new String(tail.map{_.toByte}.toArray)
        val friends = s.split("\0").toList map {_.toCharArray.toList} map {
          case char :: name => char.toInt -> new String(name.toArray)
          case _ => 0 -> ""
        }
        FriendList(crcId, sid, token, error, fragNumber, friends)
      case _ =>
        println(s"[Parser] Message not parsed: $data")
        UnspecifiedError(crcId, sid, token, 0)
    }
  }

  private def parse(m: Message): List[Int] = m match {
    case SignIn(_, sid, token, typ, name, password, agentInfo, _) =>
      List(sid/256, sid%256, (token >> 24) & 0xFF, (token >> 16) & 0xFF, (token >> 8) & 0xFF, token & 0xFF, 0, 0, 4 + name.length + password.length + agentInfo.length, 2, typ) ++ name.toList.map{_.toInt} ++ List(0) ++ password.toList.map{_.toInt} ++ List(0) ++ agentInfo.toList.map{_.toInt}
    case UserInfo(_, sid, token, _, _, _, _, _, _) =>
      List(sid/256, sid%256, (token >> 24) & 0xFF, (token >> 16) & 0xFF, (token >> 8) & 0xFF, token & 0xFF, 0, 0, 1, 4)
    case Attack(_, sid, token, arg, _, _) =>
      List(sid/256, sid%256, (token >> 24) & 0xFF, (token >> 16) & 0xFF, (token >> 8) & 0xFF, token & 0xFF, 0, 0, 2, 6, arg)
    case RangeOfProducts(_, sid, token, _, _) =>
      List(sid/256, sid%256, (token >> 24) & 0xFF, (token >> 16) & 0xFF, (token >> 8) & 0xFF, token & 0xFF, 0, 0, 1, 13)
    case Move(_, sid, token, dir, _) =>
      List(sid/256, sid%256, (token >> 24) & 0xFF, (token >> 16) & 0xFF, (token >> 8) & 0xFF, token & 0xFF, 0, 0, 2, 19, dir)
    case FriendList(_, sid, token, _, _, _) =>
      List(sid/256, sid%256, (token >> 24) & 0xFF, (token >> 16) & 0xFF, (token >> 8) & 0xFF, token & 0xFF, 0, 0, 1, 33)
    case _ =>
      println(s"[Parser] Message not parsed: $m")
      Nil
  }
}
